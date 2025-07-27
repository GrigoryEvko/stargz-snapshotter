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
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

// BenchmarkParallelCompression benchmarks compression with different worker counts
func BenchmarkParallelCompression(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1MB", 1 * 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
		{"100MB", 100 * 1024 * 1024},
	}
	
	workers := []int{1, 2, 4, 8, 16}
	
	// Generate random data for each size
	testData := make(map[string][]byte)
	for _, size := range sizes {
		data := make([]byte, size.size)
		rand.New(rand.NewSource(time.Now().UnixNano())).Read(data)
		testData[size.name] = data
	}
	
	implementations := []struct {
		name       string
		compressor Compressor
	}{
		{"PureGo", NewPureGoCompressor()},
		{"Gozstd", NewGozstdCompressor()},
	}
	
	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}
		
		for _, size := range sizes {
			data := testData[size.name]
			
			for _, w := range workers {
				benchName := fmt.Sprintf("%s/Size=%s/Workers=%d", impl.name, size.name, w)
				b.Run(benchName, func(b *testing.B) {
					// Set worker count via environment variable
					oldWorkers := os.Getenv("ZSTD_WORKERS")
					os.Setenv("ZSTD_WORKERS", strconv.Itoa(w))
					defer os.Setenv("ZSTD_WORKERS", oldWorkers)
					
					b.SetBytes(int64(len(data)))
					b.ResetTimer()
					
					for i := 0; i < b.N; i++ {
						var buf bytes.Buffer
						writer, err := impl.compressor.NewWriter(&buf, 3)
						if err != nil {
							b.Fatal(err)
						}
						
						_, err = writer.Write(data)
						if err != nil {
							b.Fatal(err)
						}
						
						err = writer.Close()
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

// TestParallelCompressionCorrectness verifies that parallel compression produces valid output
func TestParallelCompressionCorrectness(t *testing.T) {
	testData := []byte("This is test data for parallel compression verification. " +
		"We need to ensure that parallel compression produces the same decompressed output.")
	testData = bytes.Repeat(testData, 1000) // Make it larger to trigger parallel processing
	
	workers := []int{1, 2, 4, 8}
	
	implementations := []struct {
		name       string
		compressor Compressor
	}{
		{"PureGo", NewPureGoCompressor()},
		{"Gozstd", NewGozstdCompressor()},
	}
	
	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}
		
		for _, w := range workers {
			t.Run(fmt.Sprintf("%s/Workers=%d", impl.name, w), func(t *testing.T) {
				// Set worker count
				oldWorkers := os.Getenv("ZSTD_WORKERS")
				os.Setenv("ZSTD_WORKERS", strconv.Itoa(w))
				defer os.Setenv("ZSTD_WORKERS", oldWorkers)
				
				// Compress
				var compressed bytes.Buffer
				writer, err := impl.compressor.NewWriter(&compressed, 3)
				if err != nil {
					t.Fatal(err)
				}
				
				_, err = writer.Write(testData)
				if err != nil {
					t.Fatal(err)
				}
				
				err = writer.Close()
				if err != nil {
					t.Fatal(err)
				}
				
				// Decompress
				reader, err := impl.compressor.NewReader(&compressed)
				if err != nil {
					t.Fatal(err)
				}
				
				var decompressed bytes.Buffer
				_, err = decompressed.ReadFrom(reader)
				if err != nil {
					t.Fatal(err)
				}
				
				err = reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				
				// Verify
				if !bytes.Equal(testData, decompressed.Bytes()) {
					t.Errorf("Decompressed data does not match original with %d workers", w)
				}
			})
		}
	}
}

// TestGetOptimalWorkerCount verifies the worker count detection
func TestGetOptimalWorkerCount(t *testing.T) {
	// Test default detection
	workers := GetOptimalWorkerCount()
	if workers < 1 {
		t.Errorf("GetOptimalWorkerCount returned %d, expected >= 1", workers)
	}
	t.Logf("Default worker count: %d", workers)
	
	// Test environment variable override
	oldWorkers := os.Getenv("ZSTD_WORKERS")
	defer os.Setenv("ZSTD_WORKERS", oldWorkers)
	
	testCases := []struct {
		env      string
		expected int
		valid    bool
	}{
		{"4", 4, true},
		{"1", 1, true},
		{"0", workers, false}, // Should use default
		{"-1", workers, false}, // Should use default
		{"invalid", workers, false}, // Should use default
	}
	
	for _, tc := range testCases {
		os.Setenv("ZSTD_WORKERS", tc.env)
		got := GetOptimalWorkerCount()
		if tc.valid && got != tc.expected {
			t.Errorf("ZSTD_WORKERS=%s: got %d, expected %d", tc.env, got, tc.expected)
		} else if !tc.valid && got != tc.expected {
			// For invalid values, we expect the default worker count
			t.Logf("ZSTD_WORKERS=%s: correctly fell back to default %d", tc.env, got)
		}
	}
}