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

//go:build zstd_benchmark || zstd_all

package testsuite

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/containerd/stargz-snapshotter/compression/zstd"
)

// BenchmarkCompression benchmarks compression performance
func BenchmarkCompression(b *testing.B) {
	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	dataSizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	compressionLevels := []int{1, 3, 11}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		for _, dataSize := range dataSizes {
			// Generate test data with some compressibility
			testData := generateCompressibleData(dataSize.size)

			for _, level := range compressionLevels {
				if level > impl.compressor.MaxCompressionLevel() {
					continue
				}

				benchName := fmt.Sprintf("%s/%s/Level%d", impl.name, dataSize.name, level)
				b.Run(benchName, func(b *testing.B) {
					b.SetBytes(int64(dataSize.size))
					b.ResetTimer()

					for i := 0; i < b.N; i++ {
						var buf bytes.Buffer
						w, err := impl.compressor.NewWriter(&buf, level)
						if err != nil {
							b.Fatal(err)
						}

						_, err = w.Write(testData)
						if err != nil {
							b.Fatal(err)
						}

						err = w.Close()
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

// BenchmarkDecompression benchmarks decompression performance
func BenchmarkDecompression(b *testing.B) {
	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	dataSizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		for _, dataSize := range dataSizes {
			// Generate and compress test data
			testData := generateCompressibleData(dataSize.size)
			
			var compressed bytes.Buffer
			w, err := impl.compressor.NewWriter(&compressed, 3)
			if err != nil {
				b.Fatal(err)
			}
			_, err = w.Write(testData)
			if err != nil {
				b.Fatal(err)
			}
			err = w.Close()
			if err != nil {
				b.Fatal(err)
			}

			compressedData := compressed.Bytes()

			benchName := fmt.Sprintf("%s/%s", impl.name, dataSize.name)
			b.Run(benchName, func(b *testing.B) {
				b.SetBytes(int64(dataSize.size))
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					r, err := impl.compressor.NewReader(bytes.NewReader(compressedData))
					if err != nil {
						b.Fatal(err)
					}

					_, err = io.Copy(io.Discard, r)
					if err != nil {
						b.Fatal(err)
					}

					r.Close()
				}
			})
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory allocation during compression
func BenchmarkMemoryUsage(b *testing.B) {
	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	// Use 1MB of data for memory benchmarks
	testData := generateCompressibleData(1024 * 1024)

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		b.Run(impl.name+"/Compression", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				w, _ := impl.compressor.NewWriter(&buf, 3)
				w.Write(testData)
				w.Close()
			}
		})

		// Prepare compressed data for decompression benchmark
		var compressed bytes.Buffer
		w, _ := impl.compressor.NewWriter(&compressed, 3)
		w.Write(testData)
		w.Close()
		compressedData := compressed.Bytes()

		b.Run(impl.name+"/Decompression", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				r, _ := impl.compressor.NewReader(bytes.NewReader(compressedData))
				io.Copy(io.Discard, r)
				r.Close()
			}
		})
	}
}

// BenchmarkParallelOperations benchmarks parallel compression/decompression
func BenchmarkParallelOperations(b *testing.B) {
	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	// Use 100KB chunks for parallel operations
	chunkSize := 100 * 1024
	testData := generateCompressibleData(chunkSize)

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		b.Run(impl.name+"/ParallelCompression", func(b *testing.B) {
			b.SetBytes(int64(chunkSize))
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					var buf bytes.Buffer
					w, err := impl.compressor.NewWriter(&buf, 3)
					if err != nil {
						b.Fatal(err)
					}
					_, err = w.Write(testData)
					if err != nil {
						b.Fatal(err)
					}
					err = w.Close()
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// TestCompressionRatios tests compression ratios for different data types
func TestCompressionRatios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compression ratio tests in short mode")
	}

	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	dataTypes := []struct {
		name      string
		generator func(size int) []byte
		size      int
	}{
		{
			name:      "Random",
			generator: generateRandomData,
			size:      100 * 1024,
		},
		{
			name:      "Text",
			generator: generateTextData,
			size:      100 * 1024,
		},
		{
			name:      "JSON",
			generator: generateJSONData,
			size:      100 * 1024,
		},
		{
			name:      "Binary",
			generator: generateBinarySequence,
			size:      100 * 1024,
		},
		{
			name:      "Zeros",
			generator: func(size int) []byte { return make([]byte, size) },
			size:      100 * 1024,
		},
	}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			t.Logf("Skipping %s: libzstd not available", impl.name)
			continue
		}

		t.Run(impl.name, func(t *testing.T) {
			levels := []int{1, 3, 11}
			if impl.compressor.MaxCompressionLevel() >= 22 {
				levels = append(levels, 22)
			}

			for _, dataType := range dataTypes {
				testData := dataType.generator(dataType.size)

				t.Run(dataType.name, func(t *testing.T) {
					for _, level := range levels {
						var buf bytes.Buffer
						w, err := impl.compressor.NewWriter(&buf, level)
						if err != nil {
							t.Fatal(err)
						}

						_, err = w.Write(testData)
						if err != nil {
							t.Fatal(err)
						}

						err = w.Close()
						if err != nil {
							t.Fatal(err)
						}

						compressedSize := buf.Len()
						ratio := float64(compressedSize) * 100 / float64(dataType.size)
						
						t.Logf("Level %2d: %6d -> %6d bytes (%.1f%% ratio, %.1f%% saved)",
							level, dataType.size, compressedSize, ratio, 100-ratio)
					}
				})
			}
		})
	}
}

// TestThroughput measures compression/decompression throughput
func TestThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput tests in short mode")
	}

	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	// Use 10MB of data for throughput testing
	dataSize := 10 * 1024 * 1024
	testData := generateCompressibleData(dataSize)

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		t.Run(impl.name, func(t *testing.T) {
			// Test compression throughput
			t.Run("Compression", func(t *testing.T) {
				levels := []int{1, 3, 11}
				if impl.compressor.MaxCompressionLevel() >= 22 {
					levels = append(levels, 22)
				}

				for _, level := range levels {
					var totalBytes int64
					var totalTime time.Duration
					iterations := 5

					for i := 0; i < iterations; i++ {
						var buf bytes.Buffer
						start := time.Now()

						w, err := impl.compressor.NewWriter(&buf, level)
						if err != nil {
							t.Fatal(err)
						}

						n, err := w.Write(testData)
						if err != nil {
							t.Fatal(err)
						}

						err = w.Close()
						if err != nil {
							t.Fatal(err)
						}

						elapsed := time.Since(start)
						totalBytes += int64(n)
						totalTime += elapsed
					}

					throughputMBps := float64(totalBytes) / totalTime.Seconds() / 1024 / 1024
					t.Logf("Level %2d: %.2f MB/s", level, throughputMBps)
				}
			})

			// Test decompression throughput
			t.Run("Decompression", func(t *testing.T) {
				// Compress data first
				var compressed bytes.Buffer
				w, _ := impl.compressor.NewWriter(&compressed, 3)
				w.Write(testData)
				w.Close()
				compressedData := compressed.Bytes()

				var totalBytes int64
				var totalTime time.Duration
				iterations := 10

				for i := 0; i < iterations; i++ {
					start := time.Now()

					r, err := impl.compressor.NewReader(bytes.NewReader(compressedData))
					if err != nil {
						t.Fatal(err)
					}

					n, err := io.Copy(io.Discard, r)
					if err != nil {
						t.Fatal(err)
					}

					r.Close()

					elapsed := time.Since(start)
					totalBytes += n
					totalTime += elapsed
				}

				throughputMBps := float64(totalBytes) / totalTime.Seconds() / 1024 / 1024
				t.Logf("Decompression: %.2f MB/s", throughputMBps)
			})
		})
	}
}

// Helper functions for data generation
func generateCompressibleData(size int) []byte {
	data := make([]byte, size)
	rand.Seed(1) // Fixed seed for reproducibility
	
	// Mix of repetitive and random data for realistic compressibility
	pattern := []byte("The quick brown fox jumps over the lazy dog. ")
	patternLen := len(pattern)
	
	for i := 0; i < size; i++ {
		if rand.Float32() < 0.7 { // 70% pattern
			data[i] = pattern[i%patternLen]
		} else { // 30% random
			data[i] = byte(rand.Intn(256))
		}
	}
	
	return data
}

func generateRandomData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func generateTextData(size int) []byte {
	words := []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog", 
		"and", "then", "runs", "away", "quickly", "through", "forest", "path"}
	
	var buf bytes.Buffer
	for buf.Len() < size {
		word := words[rand.Intn(len(words))]
		buf.WriteString(word)
		if rand.Float32() < 0.1 {
			buf.WriteByte('\n')
		} else {
			buf.WriteByte(' ')
		}
	}
	
	return buf.Bytes()[:size]
}

func generateJSONData(size int) []byte {
	var buf bytes.Buffer
	buf.WriteString(`{"data":[`)
	
	for buf.Len() < size-100 {
		fmt.Fprintf(&buf, `{"id":%d,"name":"item_%d","value":%f},`,
			rand.Intn(1000), rand.Intn(1000), rand.Float64()*100)
	}
	
	buf.WriteString(`],"status":"ok"}`)
	
	// Pad or truncate to exact size
	data := buf.Bytes()
	if len(data) > size {
		return data[:size]
	}
	
	// Pad with spaces
	result := make([]byte, size)
	copy(result, data)
	for i := len(data); i < size; i++ {
		result[i] = ' '
	}
	return result
}

func generateBinarySequence(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte((i * 7) & 0xFF)
	}
	return data
}