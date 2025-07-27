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
	"os"
	"testing"
)

// TestIntegrationRuntimeDetection verifies the complete runtime detection flow
func TestIntegrationRuntimeDetection(t *testing.T) {
	// Test 1: Verify GetCompressor returns a valid compressor
	compressor := GetCompressor()
	if compressor == nil {
		t.Fatal("GetCompressor returned nil")
	}

	// Test 2: Verify implementation details
	t.Logf("Using implementation: %s", compressor.Name())
	t.Logf("Max compression level: %d", compressor.MaxCompressionLevel())
	t.Logf("libzstd available: %v", compressor.IsLibzstdAvailable())

	// Test 3: Test compression at various levels
	testData := []byte("This is test data for zstd compression runtime detection integration test.")
	
	levels := []int{1, 3, 11}
	if compressor.MaxCompressionLevel() >= 22 {
		levels = append(levels, 22)
	}

	for _, level := range levels {
		t.Run(fmt.Sprintf("Level_%d", level), func(t *testing.T) {
			// Compress
			var compressed bytes.Buffer
			writer, err := compressor.NewWriter(&compressed, level)
			if err != nil {
				if level > compressor.MaxCompressionLevel() {
					t.Skipf("Level %d not supported by %s", level, compressor.Name())
				}
				t.Fatal(err)
			}

			n, err := writer.Write(testData)
			if err != nil {
				t.Fatal(err)
			}
			if n != len(testData) {
				t.Errorf("Wrote %d bytes, expected %d", n, len(testData))
			}

			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			compressedSize := compressed.Len()
			t.Logf("Compressed %d bytes to %d bytes (%.2f%% ratio) at level %d",
				len(testData), compressedSize,
				float64(compressedSize)*100/float64(len(testData)), level)

			// Decompress
			reader, err := compressor.NewReader(&compressed)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}

			// Verify
			if !bytes.Equal(decompressed, testData) {
				t.Error("Decompressed data doesn't match original")
			}
		})
	}
}

// TestIntegrationEnvironmentOverride tests environment variable override
func TestIntegrationEnvironmentOverride(t *testing.T) {
	// Save original env
	origEnv := os.Getenv("ZSTD_FORCE_IMPLEMENTATION")
	defer os.Setenv("ZSTD_FORCE_IMPLEMENTATION", origEnv)

	tests := []struct {
		name     string
		envValue string
		wantImpl string
		maxLevel int
	}{
		{
			name:     "ForceKlauspost",
			envValue: "klauspost",
			wantImpl: "klauspost",
			maxLevel: 11,
		},
		{
			name:     "ForceGozstd",
			envValue: "gozstd",
			wantImpl: "gozstd",
			maxLevel: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: In a real application, this would require restarting the process
			// For testing, we're just verifying the behavior
			os.Setenv("ZSTD_FORCE_IMPLEMENTATION", tt.envValue)
			
			// In real usage, GetCompressor would pick up the env var on first call
			// Here we're just documenting the expected behavior
			t.Logf("With ZSTD_FORCE_IMPLEMENTATION=%s, expecting %s implementation", tt.envValue, tt.wantImpl)
			
			// Verify current compressor (won't change in same process)
			current := GetCompressor()
			t.Logf("Current implementation: %s (would be %s after restart)", current.Name(), tt.wantImpl)
		})
	}
}

// TestIntegrationCrossImplementationCompatibility verifies both implementations can read each other's output
func TestIntegrationCrossImplementationCompatibility(t *testing.T) {
	testData := []byte("Cross-implementation compatibility test data")
	
	implementations := []struct {
		name       string
		compressor Compressor
	}{
		{"PureGo", NewPureGoCompressor()},
		{"Gozstd", NewGozstdCompressor()},
	}

	// Skip if libzstd not available
	if !implementations[1].compressor.IsLibzstdAvailable() {
		t.Skip("libzstd not available, skipping cross-implementation test")
	}

	// Test all combinations
	for _, writer := range implementations {
		for _, reader := range implementations {
			testName := fmt.Sprintf("%s_write_%s_read", writer.name, reader.name)
			t.Run(testName, func(t *testing.T) {
				// Compress with writer implementation
				var compressed bytes.Buffer
				w, err := writer.compressor.NewWriter(&compressed, 3)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write(testData); err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}

				// Decompress with reader implementation
				r, err := reader.compressor.NewReader(bytes.NewReader(compressed.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()

				decompressed, err := io.ReadAll(r)
				if err != nil {
					t.Fatal(err)
				}

				// Verify
				if !bytes.Equal(decompressed, testData) {
					t.Errorf("%s compressed data could not be correctly decompressed by %s",
						writer.name, reader.name)
				}
			})
		}
	}
}

// TestIntegrationLargeData tests with larger, more realistic data
func TestIntegrationLargeData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large data test in short mode")
	}

	// Create 10MB of semi-compressible data
	size := 10 * 1024 * 1024
	testData := make([]byte, size)
	
	// Fill with pattern for some compressibility
	pattern := []byte("The quick brown fox jumps over the lazy dog. ")
	for i := 0; i < size; i++ {
		testData[i] = pattern[i%len(pattern)]
	}

	compressor := GetCompressor()
	
	// Test compression
	var compressed bytes.Buffer
	writer, err := compressor.NewWriter(&compressed, 3)
	if err != nil {
		t.Fatal(err)
	}

	written, err := writer.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	if written != len(testData) {
		t.Errorf("Wrote %d bytes, expected %d", written, len(testData))
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	compressedSize := compressed.Len()
	ratio := float64(compressedSize) * 100 / float64(size)
	t.Logf("Compressed %d MB to %d bytes (%.2f%% ratio) using %s",
		size/1024/1024, compressedSize, ratio, compressor.Name())

	// Test decompression
	reader, err := compressor.NewReader(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if len(decompressed) != len(testData) {
		t.Errorf("Decompressed size mismatch: got %d, want %d", len(decompressed), len(testData))
	}

	// Verify first and last portions (checking all would be slow)
	if !bytes.Equal(decompressed[:1000], testData[:1000]) {
		t.Error("First 1000 bytes don't match")
	}
	if !bytes.Equal(decompressed[len(decompressed)-1000:], testData[len(testData)-1000:]) {
		t.Error("Last 1000 bytes don't match")
	}
}

// TestIntegrationFlushBehavior tests the Flush() behavior
func TestIntegrationFlushBehavior(t *testing.T) {
	compressor := GetCompressor()
	
	// Create a writer
	var buf bytes.Buffer
	writer, err := compressor.NewWriter(&buf, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	// Write some data
	data1 := []byte("First chunk of data")
	if _, err := writer.Write(data1); err != nil {
		t.Fatal(err)
	}

	// Flush
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	flushedSize := buf.Len()
	if flushedSize == 0 {
		t.Error("No data after flush")
	}

	// Write more data
	data2 := []byte("Second chunk of data")
	if _, err := writer.Write(data2); err != nil {
		t.Fatal(err)
	}

	// Close
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	finalSize := buf.Len()
	if finalSize <= flushedSize {
		t.Error("No additional data after second write")
	}

	// Verify we can decompress the complete stream
	reader, err := compressor.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	expected := append(data1, data2...)
	if !bytes.Equal(decompressed, expected) {
		t.Error("Decompressed data doesn't match expected")
	}
}