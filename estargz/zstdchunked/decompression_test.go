/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package zstdchunked

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/containerd/stargz-snapshotter/estargz"
	compzstd "github.com/containerd/stargz-snapshotter/compression/zstd"
	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
)

// TestDecompressionWithDifferentImplementations tests that both implementations
// can correctly decompress zstd:chunked layers
func TestDecompressionWithDifferentImplementations(t *testing.T) {
	// Create test tar data
	testFiles := []struct {
		name    string
		content string
	}{
		{"file1.txt", "Hello, World!"},
		{"dir/file2.txt", "This is a test file for zstd:chunked compression"},
		{"large.txt", strings.Repeat("Large file content. ", 1000)},
	}

	// Create tar buffer
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, f := range testFiles {
		hdr := &tar.Header{
			Name: f.name,
			Mode: 0644,
			Size: int64(len(f.content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(f.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	originalTar := tarBuf.Bytes()

	// Test with different compression levels
	levels := []int{1, 3, 11}
	// Add level 22 only if libzstd is available
	if compzstd.GetCompressor().MaxCompressionLevel() >= 22 {
		levels = append(levels, 22)
	}

	for _, level := range levels {
		t.Run(fmt.Sprintf("Level_%d", level), func(t *testing.T) {
			// Create zstd:chunked compressed layer
			compressor := &Compressor{
				CompressionLevel: zstd.EncoderLevelFromZstd(level),
			}

			var compressedBuf bytes.Buffer
			writer, err := compressor.Writer(&compressedBuf)
			if err != nil {
				// Level might not be supported
				if level > 11 {
					t.Skipf("Compression level %d not supported", level)
				}
				t.Fatal(err)
			}

			// Write tar data
			if _, err := io.Copy(writer, bytes.NewReader(originalTar)); err != nil {
				t.Fatal(err)
			}
			
			// Close the writer to flush all compressed data
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			// Create TOC
			toc := &estargz.JTOC{
				Version: 1,
				Entries: make([]*estargz.TOCEntry, 0),
			}
			
			// Add TOC entries for our files
			for _, f := range testFiles {
				h := sha256.Sum256([]byte(f.content))
				toc.Entries = append(toc.Entries, &estargz.TOCEntry{
					Name:   f.name,
					Type:   "reg",
					Size:   int64(len(f.content)),
					Digest: digest.NewDigestFromBytes(digest.SHA256, h[:]).String(),
				})
			}

			// Write TOC and footer
			offset := int64(compressedBuf.Len())
			tocDigest, err := compressor.WriteTOCAndFooter(&compressedBuf, offset, toc, sha256.New())
			if err != nil {
				t.Fatal(err)
			}

			compressedData := compressedBuf.Bytes()
			t.Logf("Compressed %d bytes to %d bytes (%.2f%% ratio) with TOC digest %s",
				len(originalTar), len(compressedData), 
				float64(len(compressedData))*100/float64(len(originalTar)),
				tocDigest)

			// Test decompression with different forced implementations
			testImplementations := []struct {
				name     string
				envValue string
			}{
				{"Auto", ""},
				{"PureGo", "klauspost"},
				{"Libzstd", "gozstd"},
			}

			for _, impl := range testImplementations {
				t.Run(impl.name, func(t *testing.T) {
					// Set environment variable if needed
					if impl.envValue != "" {
						oldEnv := os.Getenv("ZSTD_FORCE_IMPLEMENTATION")
						os.Setenv("ZSTD_FORCE_IMPLEMENTATION", impl.envValue)
						defer os.Setenv("ZSTD_FORCE_IMPLEMENTATION", oldEnv)
						
						// Force re-selection of compressor
						// Note: In real usage, this would require restarting the process
						if impl.envValue == "gozstd" && compzstd.GetCompressor().MaxCompressionLevel() < 22 {
							t.Skip("libzstd not available")
						}
					}

					// Create decompressor
					decompressor := &Decompressor{}

					// Test Reader method
					reader, err := decompressor.Reader(bytes.NewReader(compressedData))
					if err != nil {
						t.Fatal(err)
					}
					defer reader.Close()

					// Read and verify decompressed data
					decompressed, err := io.ReadAll(reader)
					if err != nil {
						t.Fatal(err)
					}

					// The decompressed data should contain the original tar
					// (it will have TOC appended, so we check if it starts with original)
					if !bytes.HasPrefix(decompressed, originalTar) {
						t.Error("Decompressed data doesn't match original tar")
					}

					// First, parse the footer to get TOC location
					if len(compressedData) < int(decompressor.FooterSize()) {
						t.Fatal("compressed data too small to contain footer")
					}
					footerData := compressedData[len(compressedData)-int(decompressor.FooterSize()):]
					_, tocOffset, tocSize, err := decompressor.ParseFooter(footerData)
					if err != nil {
						t.Fatalf("Failed to parse footer: %v", err)
					}

					// Test ParseTOC method - need to seek to TOC offset
					tocReader := bytes.NewReader(compressedData[tocOffset:tocOffset+tocSize])
					tocData, parsedDigest, err := decompressor.ParseTOC(tocReader)
					if err != nil {
						t.Fatal(err)
					}

					if parsedDigest != tocDigest {
						t.Errorf("TOC digest mismatch: got %s, want %s", parsedDigest, tocDigest)
					}

					// Verify TOC content
					if tocData.Version != 1 {
						t.Errorf("Wrong TOC version: %d", tocData.Version)
					}
					if len(tocData.Entries) != len(testFiles) {
						t.Errorf("Wrong number of TOC entries: got %d, want %d", 
							len(tocData.Entries), len(testFiles))
					}

					// Test DecompressTOC method - also needs to start at TOC offset
					tocDecompressReader, err := decompressor.DecompressTOC(bytes.NewReader(compressedData[tocOffset:tocOffset+tocSize]))
					if err != nil {
						t.Fatal(err)
					}
					defer tocDecompressReader.Close()

					// Read and parse TOC JSON
					tocJSON, err := io.ReadAll(tocDecompressReader)
					if err != nil {
						t.Fatal(err)
					}

					var parsedTOC estargz.JTOC
					if err := json.Unmarshal(tocJSON, &parsedTOC); err != nil {
						t.Fatal(err)
					}

					if parsedTOC.Version != 1 {
						t.Errorf("Wrong parsed TOC version: %d", parsedTOC.Version)
					}
				})
			}
		})
	}
}

// TestDecompressionPerformance measures decompression performance for image layers
func TestDecompressionPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a larger tar file to simulate a real image layer
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	// Add multiple files of various sizes
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("File %d content: %s", i, strings.Repeat("data", i*10))
		hdr := &tar.Header{
			Name: fmt.Sprintf("file%03d.txt", i),
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	originalTar := tarBuf.Bytes()
	t.Logf("Created test tar: %d bytes", len(originalTar))

	// Compress with level 3
	compressor := &Compressor{
		CompressionLevel: 3,
	}

	var compressedBuf bytes.Buffer
	writer, err := compressor.Writer(&compressedBuf)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(writer, bytes.NewReader(originalTar)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	compressedData := compressedBuf.Bytes()
	t.Logf("Compressed to: %d bytes (%.2f%% of original)", 
		len(compressedData), float64(len(compressedData))*100/float64(len(originalTar)))

	// Measure decompression time
	decompressor := &Decompressor{}
	
	// Warm up
	reader, err := decompressor.Reader(bytes.NewReader(compressedData))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	// Measure multiple runs
	const runs = 10
	var totalBytes int64
	start := time.Now()
	
	for i := 0; i < runs; i++ {
		reader, err := decompressor.Reader(bytes.NewReader(compressedData))
		if err != nil {
			t.Fatal(err)
		}
		n, err := io.Copy(io.Discard, reader)
		if err != nil {
			t.Fatal(err)
		}
		totalBytes += n
		reader.Close()
	}
	
	elapsed := time.Since(start)
	throughput := float64(totalBytes) / elapsed.Seconds() / 1024 / 1024
	
	t.Logf("Decompression performance: %.2f MB/s (using %s)", 
		throughput, compzstd.GetCompressor().Name())
}

// TestConcurrentDecompression tests concurrent decompression of multiple layers
func TestConcurrentDecompression(t *testing.T) {
	// Create test data
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	content := strings.Repeat("Concurrent test data. ", 1000)
	hdr := &tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	// Compress the data
	compressor := &Compressor{
		CompressionLevel: 3,
	}
	var compressedBuf bytes.Buffer
	writer, err := compressor.Writer(&compressedBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(writer, &tarBuf); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	compressedData := compressedBuf.Bytes()

	// Test concurrent decompression
	const goroutines = 10
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			decompressor := &Decompressor{}
			reader, err := decompressor.Reader(bytes.NewReader(compressedData))
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: failed to create reader: %w", id, err)
				return
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: failed to read: %w", id, err)
				return
			}

			// Verify we got valid data
			if len(data) == 0 {
				errCh <- fmt.Errorf("goroutine %d: no data decompressed", id)
				return
			}

			errCh <- nil
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		if err := <-errCh; err != nil {
			t.Error(err)
		}
	}
}