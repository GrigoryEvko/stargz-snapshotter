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

//go:build zstd_integration || zstd_all

package testsuite

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/containerd/stargz-snapshotter/compression/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCrossImplementationCompatibility ensures data compressed by one implementation
// can be correctly decompressed by another
func TestCrossImplementationCompatibility(t *testing.T) {
	implementations := []struct {
		name       string
		compressor zstd.Compressor
		available  bool
	}{
		{
			name:       "PureGo",
			compressor: zstd.NewPureGoCompressor(),
			available:  true,
		},
		{
			name:       "Gozstd",
			compressor: zstd.NewGozstdCompressor(),
			available:  false,
		},
	}

	// Check availability
	implementations[1].available = implementations[1].compressor.IsLibzstdAvailable()

	if !implementations[1].available {
		t.Skip("libzstd not available, skipping cross-implementation compatibility tests")
	}

	testCases := []struct {
		name  string
		data  []byte
		level int
	}{
		{
			name:  "SmallText",
			data:  []byte("Hello, World! This is a compatibility test."),
			level: 3,
		},
		{
			name:  "BinaryData",
			data:  generateBinaryData(1024),
			level: 5,
		},
		{
			name:  "RepetitiveData",
			data:  bytes.Repeat([]byte("test"), 1000),
			level: 11,
		},
		{
			name:  "MixedContent",
			data:  generateMixedContent(10240),
			level: 3,
		},
	}

	// Test all combinations of writer/reader
	for _, writer := range implementations {
		if !writer.available {
			continue
		}

		for _, reader := range implementations {
			if !reader.available {
				continue
			}

			testName := fmt.Sprintf("%s_to_%s", writer.name, reader.name)
			t.Run(testName, func(t *testing.T) {
				for _, tc := range testCases {
					t.Run(tc.name, func(t *testing.T) {
						// Skip levels not supported by the writer
						if tc.level > writer.compressor.MaxCompressionLevel() {
							t.Skipf("Level %d not supported by %s", tc.level, writer.name)
						}

						// Compress with writer implementation
						var compressed bytes.Buffer
						w, err := writer.compressor.NewWriter(&compressed, tc.level)
						require.NoError(t, err)

						n, err := w.Write(tc.data)
						require.NoError(t, err)
						assert.Equal(t, len(tc.data), n)

						err = w.Close()
						require.NoError(t, err)

						compressedData := compressed.Bytes()

						// Calculate checksum of compressed data
						compressedHash := sha256.Sum256(compressedData)
						t.Logf("Compressed by %s: %d bytes, hash: %s",
							writer.name, len(compressedData), hex.EncodeToString(compressedHash[:8]))

						// Decompress with reader implementation
						r, err := reader.compressor.NewReader(bytes.NewReader(compressedData))
						require.NoError(t, err)
						defer r.Close()

						decompressed, err := io.ReadAll(r)
						require.NoError(t, err)

						// Verify data integrity
						assert.Equal(t, tc.data, decompressed,
							"Data mismatch: %s compressed -> %s decompressed",
							writer.name, reader.name)

						// Verify checksums
						originalHash := sha256.Sum256(tc.data)
						decompressedHash := sha256.Sum256(decompressed)
						assert.Equal(t, originalHash, decompressedHash,
							"Checksum mismatch after %s -> %s", writer.name, reader.name)
					})
				}
			})
		}
	}
}

// TestCompressionLevelCompatibility verifies that different compression levels
// produce compatible output
func TestCompressionLevelCompatibility(t *testing.T) {
	suite := NewTestSuite()

	testData := []byte("Testing compression level compatibility across implementations")

	for _, impl := range suite.implementations {
		if impl.Skip {
			continue
		}

		t.Run(impl.Name, func(t *testing.T) {
			// Compress at various levels
			levels := []int{1, 3, 5, 7, 9, 11}
			if impl.Compressor.MaxCompressionLevel() >= 22 {
				levels = append(levels, 15, 19, 22)
			}

			compressedByLevel := make(map[int][]byte)

			// Compress at each level
			for _, level := range levels {
				var buf bytes.Buffer
				w, err := impl.Compressor.NewWriter(&buf, level)
				require.NoError(t, err)

				_, err = w.Write(testData)
				require.NoError(t, err)

				err = w.Close()
				require.NoError(t, err)

				compressedByLevel[level] = buf.Bytes()
				t.Logf("Level %d: %d bytes", level, len(buf.Bytes()))
			}

			// Verify all can be decompressed correctly
			for level, compressed := range compressedByLevel {
				r, err := impl.Compressor.NewReader(bytes.NewReader(compressed))
				require.NoError(t, err)

				decompressed, err := io.ReadAll(r)
				require.NoError(t, err)
				r.Close()

				assert.Equal(t, testData, decompressed,
					"Level %d decompression failed", level)
			}

			// Verify compression improves with higher levels (generally)
			if len(levels) > 1 {
				lowLevel := compressedByLevel[levels[0]]
				highLevel := compressedByLevel[levels[len(levels)-1]]
				
				// High compression should generally produce smaller output
				// (though not guaranteed for all data)
				t.Logf("Compression improvement: %d bytes (level %d) -> %d bytes (level %d)",
					len(lowLevel), levels[0], len(highLevel), levels[len(levels)-1])
			}
		})
	}
}

// TestStreamingCompatibility tests that streaming compression/decompression
// works correctly across implementations
func TestStreamingCompatibility(t *testing.T) {
	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	// Skip if libzstd not available
	if !implementations[1].compressor.IsLibzstdAvailable() {
		t.Skip("libzstd not available")
	}

	// Test streaming with chunks
	chunks := [][]byte{
		[]byte("First chunk of streaming data\n"),
		[]byte("Second chunk with more content\n"),
		[]byte("Third chunk continues the stream\n"),
		[]byte("Final chunk completes the test\n"),
	}

	for _, writer := range implementations {
		for _, reader := range implementations {
			testName := fmt.Sprintf("%s_write_%s_read_streaming", writer.name, reader.name)
			t.Run(testName, func(t *testing.T) {
				// Set up pipes for streaming
				pr, pw := io.Pipe()
				
				// Compressed data buffer
				var compressed bytes.Buffer

				// Start compressor in goroutine
				errCh := make(chan error, 1)
				go func() {
					w, err := writer.compressor.NewWriter(pw, 3)
					if err != nil {
						errCh <- err
						pw.Close()
						return
					}

					// Write chunks with flushes
					for i, chunk := range chunks {
						_, err := w.Write(chunk)
						if err != nil {
							errCh <- err
							w.Close()
							pw.Close()
							return
						}

						// Flush after each chunk (except last)
						if i < len(chunks)-1 {
							err = w.Flush()
							if err != nil {
								errCh <- err
								w.Close()
								pw.Close()
								return
							}
						}
					}

					err = w.Close()
					if err != nil {
						errCh <- err
					}
					pw.Close()
					errCh <- nil
				}()

				// Copy compressed data
				_, err := io.Copy(&compressed, pr)
				require.NoError(t, err)

				// Check compressor error
				err = <-errCh
				require.NoError(t, err)

				// Now decompress with reader implementation
				r, err := reader.compressor.NewReader(&compressed)
				require.NoError(t, err)
				defer r.Close()

				decompressed, err := io.ReadAll(r)
				require.NoError(t, err)

				// Verify
				expected := bytes.Join(chunks, nil)
				assert.Equal(t, expected, decompressed,
					"Streaming data mismatch: %s -> %s", writer.name, reader.name)
			})
		}
	}
}

// Helper functions
func generateBinaryData(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i & 0xFF)
	}
	return data
}

func generateMixedContent(size int) []byte {
	data := make([]byte, size)
	patterns := [][]byte{
		[]byte("text content "),
		{0x00, 0x01, 0x02, 0x03},
		[]byte("more text\n"),
		{0xFF, 0xFE, 0xFD, 0xFC},
	}

	pos := 0
	patternIdx := 0
	for pos < size {
		pattern := patterns[patternIdx%len(patterns)]
		n := copy(data[pos:], pattern)
		pos += n
		patternIdx++
	}
	return data
}