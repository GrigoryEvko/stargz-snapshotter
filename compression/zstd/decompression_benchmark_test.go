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

package zstd

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

// BenchmarkDecompression compares decompression performance
func BenchmarkDecompression(b *testing.B) {
	// Test data sizes
	dataSizes := []int{
		1024,       // 1KB
		10240,      // 10KB
		102400,     // 100KB
		1048576,    // 1MB
		10485760,   // 10MB
	}

	// Compression levels to test
	compressionLevels := []int{1, 3, 11, 22}

	for _, size := range dataSizes {
		// Generate test data with some compressibility
		testData := generateCompressibleData(size)

		for _, level := range compressionLevels {
			// Try to compress with both implementations
			compressors := []struct {
				name string
				comp Compressor
			}{
				{"PureGo", NewPureGoCompressor()},
				{"Gozstd", NewGozstdCompressor()},
			}

			for _, c := range compressors {
				// Skip if implementation not available or level not supported
				if !c.comp.IsLibzstdAvailable() && c.name == "Gozstd" {
					continue
				}
				if level > c.comp.MaxCompressionLevel() {
					continue
				}

				// Compress the data first
				var compressed bytes.Buffer
				writer, err := c.comp.NewWriter(&compressed, level)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := writer.Write(testData); err != nil {
					b.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					b.Fatal(err)
				}

				compressedData := compressed.Bytes()

				// Benchmark decompression
				benchName := fmt.Sprintf("%s/Size=%s/Level=%d", c.name, formatSize(size), level)
				b.Run(benchName, func(b *testing.B) {
					b.SetBytes(int64(len(testData)))
					b.ResetTimer()

					for i := 0; i < b.N; i++ {
						reader, err := c.comp.NewReader(bytes.NewReader(compressedData))
						if err != nil {
							b.Fatal(err)
						}
						
						// Decompress and discard
						if _, err := io.Copy(io.Discard, reader); err != nil {
							b.Fatal(err)
						}
						reader.Close()
					}
				})
			}
		}
	}
}

// BenchmarkDecompressionParallel tests parallel decompression performance
func BenchmarkDecompressionParallel(b *testing.B) {
	// Use 1MB of compressible data
	size := 1048576
	testData := generateCompressibleData(size)

	compressors := []struct {
		name string
		comp Compressor
	}{
		{"PureGo", NewPureGoCompressor()},
		{"Gozstd", NewGozstdCompressor()},
	}

	for _, c := range compressors {
		if !c.comp.IsLibzstdAvailable() && c.name == "Gozstd" {
			continue
		}

		// Compress at level 3
		var compressed bytes.Buffer
		writer, err := c.comp.NewWriter(&compressed, 3)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := writer.Write(testData); err != nil {
			b.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			b.Fatal(err)
		}

		compressedData := compressed.Bytes()

		b.Run(c.name, func(b *testing.B) {
			b.SetBytes(int64(len(testData)))
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					reader, err := c.comp.NewReader(bytes.NewReader(compressedData))
					if err != nil {
						b.Fatal(err)
					}
					
					if _, err := io.Copy(io.Discard, reader); err != nil {
						b.Fatal(err)
					}
					reader.Close()
				}
			})
		})
	}
}

// BenchmarkStreamingDecompression tests streaming decompression performance
func BenchmarkStreamingDecompression(b *testing.B) {
	// Use 10MB of data to test streaming
	size := 10485760
	testData := generateCompressibleData(size)

	compressors := []struct {
		name string
		comp Compressor
	}{
		{"PureGo", NewPureGoCompressor()},
		{"Gozstd", NewGozstdCompressor()},
	}

	for _, c := range compressors {
		if !c.comp.IsLibzstdAvailable() && c.name == "Gozstd" {
			continue
		}

		// Compress the data
		var compressed bytes.Buffer
		writer, err := c.comp.NewWriter(&compressed, 3)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := writer.Write(testData); err != nil {
			b.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			b.Fatal(err)
		}

		compressedData := compressed.Bytes()

		b.Run(c.name, func(b *testing.B) {
			b.SetBytes(int64(len(testData)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Simulate streaming by using a pipe
				pr, pw := io.Pipe()
				
				// Writer goroutine
				go func() {
					_, err := io.Copy(pw, bytes.NewReader(compressedData))
					pw.CloseWithError(err)
				}()

				// Reader (decompressor)
				reader, err := c.comp.NewReader(pr)
				if err != nil {
					b.Fatal(err)
				}
				
				if _, err := io.Copy(io.Discard, reader); err != nil {
					b.Fatal(err)
				}
				reader.Close()
			}
		})
	}
}

// Helper function to generate compressible data
func generateCompressibleData(size int) []byte {
	data := make([]byte, size)
	rand.Seed(1) // Fixed seed for reproducibility
	
	// Fill with somewhat repetitive data for compressibility
	pattern := []byte("The quick brown fox jumps over the lazy dog. ")
	patternLen := len(pattern)
	
	for i := 0; i < size; i++ {
		if rand.Float32() < 0.7 { // 70% pattern, 30% random
			data[i] = pattern[i%patternLen]
		} else {
			data[i] = byte(rand.Intn(256))
		}
	}
	
	return data
}

// Helper function to format size
func formatSize(size int) string {
	switch {
	case size >= 1048576:
		return fmt.Sprintf("%dMB", size/1048576)
	case size >= 1024:
		return fmt.Sprintf("%dKB", size/1024)
	default:
		return fmt.Sprintf("%dB", size)
	}
}