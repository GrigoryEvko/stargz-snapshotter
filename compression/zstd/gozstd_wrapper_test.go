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

func TestGozstdCompressor_IsAvailable(t *testing.T) {
	defer SetupSingleThreadedTest(t)()
	compressor := &gozstdCompressor{}
	// This will return false if libzstd is not available or if testing fails
	// We just verify it returns a boolean without panicking
	_ = compressor.IsLibzstdAvailable()
}

func TestGozstdCompressor_CompressionDecompression(t *testing.T) {
	defer SetupSingleThreadedTest(t)()
	compressor := &gozstdCompressor{}
	
	// Skip if libzstd is not available
	if !compressor.tryLibzstd() {
		t.Skip("libzstd not available, skipping gozstd tests")
	}
	
	testData := []byte("Hello, World! This is a test of zstd compression.")
	levels := []int{1, 3, 11, 22}
	
	for _, level := range levels {
		t.Run(fmt.Sprintf("Level_%d", level), func(t *testing.T) {
			// Compress
			var compressed bytes.Buffer
			writer, err := compressor.NewWriter(&compressed, level)
			if err != nil {
				// Level might not be supported
				if level > compressor.MaxCompressionLevel() {
					t.Skipf("Level %d exceeds maximum %d", level, compressor.MaxCompressionLevel())
				}
				t.Fatalf("Failed to create writer: %v", err)
			}
			
			n, err := writer.Write(testData)
			if err != nil {
				t.Fatalf("Failed to write data: %v", err)
			}
			if n != len(testData) {
				t.Errorf("Written bytes mismatch: got %d, want %d", n, len(testData))
			}
			
			if err := writer.Close(); err != nil {
				t.Fatalf("Failed to close writer: %v", err)
			}
			
			// Decompress
			reader, err := compressor.NewReader(bytes.NewReader(compressed.Bytes()))
			if err != nil {
				t.Fatalf("Failed to create reader: %v", err)
			}
			defer reader.Close()
			
			decompressed, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to read decompressed data: %v", err)
			}
			
			if !bytes.Equal(decompressed, testData) {
				t.Errorf("Decompressed data mismatch: got %q, want %q", decompressed, testData)
			}
		})
	}
}

func TestGozstdCompressor_MaxCompressionLevel(t *testing.T) {
	defer SetupSingleThreadedTest(t)()
	compressor := &gozstdCompressor{}
	maxLevel := compressor.MaxCompressionLevel()
	
	// When libzstd is available, max level should be 22
	// When not available, it falls back to 11
	if compressor.tryLibzstd() {
		if maxLevel != 22 {
			t.Errorf("Expected max level 22 with libzstd, got %d", maxLevel)
		}
	} else {
		if maxLevel != 11 {
			t.Errorf("Expected max level 11 without libzstd, got %d", maxLevel)
		}
	}
}

func TestGozstdCompressor_Name(t *testing.T) {
	defer SetupSingleThreadedTest(t)()
	compressor := &gozstdCompressor{}
	name := compressor.Name()
	
	if compressor.tryLibzstd() {
		if name != "gozstd (libzstd)" {
			t.Errorf("Expected name 'gozstd (libzstd)', got %q", name)
		}
	} else {
		if name != "gozstd (fallback to pure-go)" {
			t.Errorf("Expected name 'gozstd (fallback to pure-go)', got %q", name)
		}
	}
}