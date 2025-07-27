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

//go:build zstd_stress || zstd_all

package testsuite

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/containerd/stargz-snapshotter/compression/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentOperations tests concurrent compression/decompression
func TestConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		t.Run(impl.name, func(t *testing.T) {
			// Number of concurrent goroutines
			concurrency := runtime.NumCPU() * 2
			operations := 100
			dataSize := 100 * 1024 // 100KB per operation

			var wg sync.WaitGroup
			errCh := make(chan error, concurrency*operations)
			var successCount int64

			// Generate different data for each goroutine
			testData := make([][]byte, concurrency)
			for i := 0; i < concurrency; i++ {
				testData[i] = generateStressTestData(dataSize, i)
			}

			start := time.Now()

			// Launch concurrent operations
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					data := testData[workerID]
					
					for j := 0; j < operations; j++ {
						// Compress
						var compressed bytes.Buffer
						w, err := impl.compressor.NewWriter(&compressed, 3)
						if err != nil {
							errCh <- fmt.Errorf("worker %d: compression init failed: %v", workerID, err)
							continue
						}

						_, err = w.Write(data)
						if err != nil {
							errCh <- fmt.Errorf("worker %d: write failed: %v", workerID, err)
							w.Close()
							continue
						}

						err = w.Close()
						if err != nil {
							errCh <- fmt.Errorf("worker %d: close failed: %v", workerID, err)
							continue
						}

						// Decompress
						r, err := impl.compressor.NewReader(&compressed)
						if err != nil {
							errCh <- fmt.Errorf("worker %d: decompression init failed: %v", workerID, err)
							continue
						}

						decompressed, err := io.ReadAll(r)
						r.Close()
						if err != nil {
							errCh <- fmt.Errorf("worker %d: read failed: %v", workerID, err)
							continue
						}

						// Verify
						if !bytes.Equal(data, decompressed) {
							errCh <- fmt.Errorf("worker %d: data mismatch", workerID)
							continue
						}

						atomic.AddInt64(&successCount, 1)
					}
				}(i)
			}

			wg.Wait()
			close(errCh)

			elapsed := time.Since(start)

			// Check for errors
			var errors []error
			for err := range errCh {
				errors = append(errors, err)
			}

			if len(errors) > 0 {
				t.Errorf("Encountered %d errors during concurrent operations", len(errors))
				for i, err := range errors[:min(5, len(errors))] {
					t.Errorf("Error %d: %v", i+1, err)
				}
				if len(errors) > 5 {
					t.Errorf("... and %d more errors", len(errors)-5)
				}
			}

			totalOperations := int64(concurrency * operations)
			successRate := float64(successCount) * 100 / float64(totalOperations)
			throughputMBps := float64(successCount*int64(dataSize)) / elapsed.Seconds() / 1024 / 1024

			t.Logf("Concurrent operations completed in %v", elapsed)
			t.Logf("Success rate: %.2f%% (%d/%d)", successRate, successCount, totalOperations)
			t.Logf("Throughput: %.2f MB/s", throughputMBps)

			assert.Equal(t, totalOperations, successCount, "All operations should succeed")
		})
	}
}

// TestMemoryStress tests memory usage under stress
func TestMemoryStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stress test in short mode")
	}

	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		t.Run(impl.name, func(t *testing.T) {
			// Force GC before starting
			runtime.GC()
			runtime.GC()
			
			var memStart runtime.MemStats
			runtime.ReadMemStats(&memStart)

			// Create many compressors and process data
			numInstances := 100
			dataSize := 1024 * 1024 // 1MB per instance
			
			var wg sync.WaitGroup
			for i := 0; i < numInstances; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					
					data := make([]byte, dataSize)
					rand.Read(data)
					
					// Compress
					var compressed bytes.Buffer
					w, err := impl.compressor.NewWriter(&compressed, 3)
					require.NoError(t, err)
					
					_, err = w.Write(data)
					require.NoError(t, err)
					
					err = w.Close()
					require.NoError(t, err)
					
					// Decompress
					r, err := impl.compressor.NewReader(&compressed)
					require.NoError(t, err)
					
					_, err = io.Copy(io.Discard, r)
					require.NoError(t, err)
					
					r.Close()
				}()
			}
			
			wg.Wait()
			
			// Force GC and measure memory
			runtime.GC()
			runtime.GC()
			
			var memEnd runtime.MemStats
			runtime.ReadMemStats(&memEnd)
			
			// Use current allocation since GC might have reduced memory below start
			memUsedMB := float64(memEnd.Alloc) / 1024 / 1024
			t.Logf("Memory used: %.2f MB", memUsedMB)
			t.Logf("Total allocations: %d", memEnd.TotalAlloc-memStart.TotalAlloc)
			t.Logf("Number of GC runs: %d", memEnd.NumGC-memStart.NumGC)
		})
	}
}

// TestLargeFileStress tests handling of very large files
func TestLargeFileStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file stress test in short mode")
	}

	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		t.Run(impl.name, func(t *testing.T) {
			// Test with 100MB file
			fileSize := 100 * 1024 * 1024
			
			// Generate data in chunks to avoid OOM
			chunkSize := 10 * 1024 * 1024 // 10MB chunks
			numChunks := fileSize / chunkSize
			
			// Create a pipe for streaming
			pr, pw := io.Pipe()
			
			// Compress in goroutine
			var compressed bytes.Buffer
			errCh := make(chan error, 1)
			
			go func() {
				w, err := impl.compressor.NewWriter(pw, 3)
				if err != nil {
					errCh <- err
					pw.Close()
					return
				}
				
				// Write data in chunks
				for i := 0; i < numChunks; i++ {
					chunk := generateStressTestData(chunkSize, i)
					_, err := w.Write(chunk)
					if err != nil {
						errCh <- err
						w.Close()
						pw.Close()
						return
					}
				}
				
				err = w.Close()
				if err != nil {
					errCh <- err
				}
				pw.Close()
				errCh <- nil
			}()
			
			// Read compressed data
			start := time.Now()
			n, err := io.Copy(&compressed, pr)
			require.NoError(t, err)
			
			err = <-errCh
			require.NoError(t, err)
			
			compressionTime := time.Since(start)
			compressedSize := compressed.Len()
			compressionRatio := float64(compressedSize) * 100 / float64(fileSize)
			
			t.Logf("Compressed %d MB to %d MB (%.2f%%) in %v",
				fileSize/1024/1024, compressedSize/1024/1024, compressionRatio, compressionTime)
			t.Logf("Compression speed: %.2f MB/s", 
				float64(fileSize)/compressionTime.Seconds()/1024/1024)
			
			// Save compressed size before reader consumes the buffer
			assert.Greater(t, compressedSize, 0, "Should have compressed data")
			assert.Equal(t, n, int64(compressedSize), "Bytes copied should match buffer size")
			
			// Decompress and verify size
			start = time.Now()
			r, err := impl.compressor.NewReader(&compressed)
			require.NoError(t, err)
			
			decompressedSize, err := io.Copy(io.Discard, r)
			require.NoError(t, err)
			r.Close()
			
			decompressionTime := time.Since(start)
			
			assert.Equal(t, int64(fileSize), decompressedSize, "Decompressed size should match original")
			
			t.Logf("Decompression speed: %.2f MB/s",
				float64(fileSize)/decompressionTime.Seconds()/1024/1024)
		})
	}
}

// TestRapidWriterCreation tests rapid creation and destruction of writers
func TestRapidWriterCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rapid writer creation test in short mode")
	}

	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		t.Run(impl.name, func(t *testing.T) {
			iterations := 10000
			testData := []byte("Small test data for rapid writer creation")
			
			start := time.Now()
			
			for i := 0; i < iterations; i++ {
				var buf bytes.Buffer
				w, err := impl.compressor.NewWriter(&buf, 3)
				require.NoError(t, err)
				
				_, err = w.Write(testData)
				require.NoError(t, err)
				
				err = w.Close()
				require.NoError(t, err)
				
				// Verify we can read it back
				r, err := impl.compressor.NewReader(&buf)
				require.NoError(t, err)
				
				data, err := io.ReadAll(r)
				require.NoError(t, err)
				r.Close()
				
				assert.Equal(t, testData, data)
			}
			
			elapsed := time.Since(start)
			opsPerSecond := float64(iterations) / elapsed.Seconds()
			
			t.Logf("Created and used %d writer/reader pairs in %v", iterations, elapsed)
			t.Logf("Operations per second: %.0f", opsPerSecond)
		})
	}
}

// TestErrorRecovery tests error recovery scenarios
func TestErrorRecovery(t *testing.T) {
	implementations := []struct {
		name       string
		compressor zstd.Compressor
	}{
		{"PureGo", zstd.NewPureGoCompressor()},
		{"Gozstd", zstd.NewGozstdCompressor()},
	}

	for _, impl := range implementations {
		if !impl.compressor.IsLibzstdAvailable() && impl.name == "Gozstd" {
			continue
		}

		t.Run(impl.name, func(t *testing.T) {
			t.Run("PartialWrite", func(t *testing.T) {
				// Create a writer that fails after some bytes
				failAfter := 100
				fw := &failingWriter{failAfter: failAfter}
				
				w, err := impl.compressor.NewWriter(fw, 3)
				require.NoError(t, err)
				
				// Try to write more than failAfter bytes
				data := make([]byte, 1000)
				rand.Read(data)
				
				_, err = w.Write(data)
				// Write might succeed if data is buffered
				if err == nil {
					// Force flush to trigger the error
					err = w.Flush()
				}
				assert.Error(t, err, "Should fail when underlying writer fails")
				
				// Close should handle the error gracefully
				err = w.Close()
				assert.Error(t, err, "Close should propagate write error")
			})
			
			t.Run("TruncatedData", func(t *testing.T) {
				// Compress some data
				testData := []byte("This is test data that will be truncated")
				var compressed bytes.Buffer
				
				w, err := impl.compressor.NewWriter(&compressed, 3)
				require.NoError(t, err)
				
				_, err = w.Write(testData)
				require.NoError(t, err)
				
				err = w.Close()
				require.NoError(t, err)
				
				// Truncate the compressed data aggressively
				compressedData := compressed.Bytes()
				if len(compressedData) > 10 {
					// Truncate to just 2 bytes - definitely not enough for valid zstd frame
					// Zstd frame starts with magic number 0xFD2FB528 (4 bytes)
					truncated := compressedData[:2]
					
					// Try to decompress truncated data and read expected amount
					r, err := impl.compressor.NewReader(bytes.NewReader(truncated))
					if err == nil {
						// Force reading the expected amount of data
						buf := make([]byte, len(testData))
						_, err = io.ReadFull(r, buf)
						r.Close()
					}
					assert.Error(t, err, "Should fail on truncated data")
				}
			})
		})
	}
}

// Helper types and functions

type failingWriter struct {
	written   int
	failAfter int
}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	if fw.written+len(p) > fw.failAfter {
		canWrite := fw.failAfter - fw.written
		if canWrite > 0 {
			fw.written += canWrite
			return canWrite, fmt.Errorf("write failed after %d bytes", fw.failAfter)
		}
		return 0, fmt.Errorf("write failed")
	}
	fw.written += len(p)
	return len(p), nil
}

func generateStressTestData(size int, seed int) []byte {
	// Use real source code with variation for stress testing
	return GetVariedSourceData(size, seed)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}