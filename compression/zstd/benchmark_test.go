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
	"testing"
)

// Test data sizes
var testData = generateTestData()

func generateTestData() []byte {
	// Generate 1MB of pseudo-random but compressible data
	size := 1024 * 1024
	data := make([]byte, size)
	
	// Fill with repeating pattern to ensure compressibility
	pattern := []byte("The quick brown fox jumps over the lazy dog. ")
	for i := 0; i < size; i += len(pattern) {
		copy(data[i:], pattern)
	}
	
	return data
}

// Benchmark compression at different levels
func benchmarkCompression(b *testing.B, compressor Compressor, level int) {
	b.ResetTimer()
	b.SetBytes(int64(len(testData)))
	
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		writer, err := compressor.NewWriter(&buf, level)
		if err != nil {
			b.Fatal(err)
		}
		
		n, err := writer.Write(testData)
		if err != nil {
			b.Fatal(err)
		}
		if n != len(testData) {
			b.Fatalf("short write: %d/%d", n, len(testData))
		}
		
		if err := writer.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark decompression
func benchmarkDecompression(b *testing.B, compressor Compressor, level int) {
	// First compress the data
	var compressed bytes.Buffer
	writer, err := compressor.NewWriter(&compressed, level)
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
	
	b.ResetTimer()
	b.SetBytes(int64(len(testData)))
	
	for i := 0; i < b.N; i++ {
		reader, err := compressor.NewReader(bytes.NewReader(compressedData))
		if err != nil {
			b.Fatal(err)
		}
		
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			b.Fatal(err)
		}
		
		if len(decompressed) != len(testData) {
			b.Fatalf("decompressed size mismatch: %d/%d", len(decompressed), len(testData))
		}
		
		reader.Close()
	}
}

// Pure Go benchmarks
func BenchmarkPureGoCompressionLevel1(b *testing.B) {
	benchmarkCompression(b, NewPureGoCompressor(), 1)
}

func BenchmarkPureGoCompressionLevel3(b *testing.B) {
	benchmarkCompression(b, NewPureGoCompressor(), 3)
}

func BenchmarkPureGoCompressionLevel11(b *testing.B) {
	benchmarkCompression(b, NewPureGoCompressor(), 11)
}

func BenchmarkPureGoDecompressionLevel3(b *testing.B) {
	benchmarkDecompression(b, NewPureGoCompressor(), 3)
}

// Gozstd benchmarks (will skip if libzstd not available)
func BenchmarkGozstdCompressionLevel1(b *testing.B) {
	compressor := NewGozstdCompressor()
	if !compressor.IsLibzstdAvailable() {
		b.Skip("libzstd not available")
	}
	benchmarkCompression(b, compressor, 1)
}

func BenchmarkGozstdCompressionLevel3(b *testing.B) {
	compressor := NewGozstdCompressor()
	if !compressor.IsLibzstdAvailable() {
		b.Skip("libzstd not available")
	}
	benchmarkCompression(b, compressor, 3)
}

func BenchmarkGozstdCompressionLevel11(b *testing.B) {
	compressor := NewGozstdCompressor()
	if !compressor.IsLibzstdAvailable() {
		b.Skip("libzstd not available")
	}
	benchmarkCompression(b, compressor, 11)
}

func BenchmarkGozstdCompressionLevel22(b *testing.B) {
	compressor := NewGozstdCompressor()
	if !compressor.IsLibzstdAvailable() {
		b.Skip("libzstd not available")
	}
	benchmarkCompression(b, compressor, 22)
}

func BenchmarkGozstdDecompressionLevel3(b *testing.B) {
	compressor := NewGozstdCompressor()
	if !compressor.IsLibzstdAvailable() {
		b.Skip("libzstd not available")
	}
	benchmarkDecompression(b, compressor, 3)
}

// Memory usage benchmarks
func BenchmarkPureGoMemoryUsage(b *testing.B) {
	b.ReportAllocs()
	benchmarkCompression(b, NewPureGoCompressor(), 3)
}

func BenchmarkGozstdMemoryUsage(b *testing.B) {
	compressor := NewGozstdCompressor()
	if !compressor.IsLibzstdAvailable() {
		b.Skip("libzstd not available")
	}
	b.ReportAllocs()
	benchmarkCompression(b, compressor, 3)
}

// Compression ratio benchmark
func TestCompressionRatio(t *testing.T) {
	levels := []int{1, 3, 11, 22}
	
	for _, level := range levels {
		t.Run(fmt.Sprintf("PureGo_Level%d", level), func(t *testing.T) {
			if level > 11 {
				t.Skip("Pure Go only supports up to level 11")
			}
			testCompressionRatio(t, NewPureGoCompressor(), level)
		})
		
		t.Run(fmt.Sprintf("Gozstd_Level%d", level), func(t *testing.T) {
			compressor := NewGozstdCompressor()
			if !compressor.IsLibzstdAvailable() {
				t.Skip("libzstd not available")
			}
			testCompressionRatio(t, compressor, level)
		})
	}
}

func testCompressionRatio(t *testing.T, compressor Compressor, level int) {
	var compressed bytes.Buffer
	writer, err := compressor.NewWriter(&compressed, level)
	if err != nil {
		t.Fatal(err)
	}
	
	if _, err := writer.Write(testData); err != nil {
		t.Fatal(err)
	}
	
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	
	originalSize := len(testData)
	compressedSize := compressed.Len()
	ratio := float64(compressedSize) / float64(originalSize) * 100
	
	t.Logf("%s Level %d: %d -> %d bytes (%.2f%%)", 
		compressor.Name(), level, originalSize, compressedSize, ratio)
}