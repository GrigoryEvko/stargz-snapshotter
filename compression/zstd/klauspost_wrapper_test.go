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

func TestKlauspostCompressor_Properties(t *testing.T) {
	defer SetupSingleThreadedTest(t)()
	compressor := NewPureGoCompressor()
	
	// Test IsLibzstdAvailable - should always be false
	if compressor.IsLibzstdAvailable() {
		t.Error("klauspost compressor should not report libzstd as available")
	}
	
	// Test MaxCompressionLevel - should be 11
	if maxLevel := compressor.MaxCompressionLevel(); maxLevel != 11 {
		t.Errorf("Expected max compression level 11, got %d", maxLevel)
	}
	
	// Test Name
	if name := compressor.Name(); name != "pure-go (klauspost/compress)" {
		t.Errorf("Expected name 'pure-go (klauspost/compress)', got %q", name)
	}
}

func TestKlauspostCompressor_CompressionDecompression(t *testing.T) {
	defer SetupSingleThreadedTest(t)()
	compressor := NewPureGoCompressor()
	
	testData := []byte("Hello, World! This is a test of zstd compression using pure Go implementation.")
	levels := []int{1, 3, 11, 15} // 15 should be capped to 11
	
	for _, level := range levels {
		t.Run(fmt.Sprintf("Level_%d", level), func(t *testing.T) {
			// Compress
			var compressed bytes.Buffer
			writer, err := compressor.NewWriter(&compressed, level)
			if err != nil {
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

func TestKlauspostCompressor_InvalidLevel(t *testing.T) {
	defer SetupSingleThreadedTest(t)()
	compressor := NewPureGoCompressor()
	
	// Test that levels beyond 11 work (they should be capped internally)
	writer, err := compressor.NewWriter(io.Discard, 22)
	if err != nil {
		t.Fatalf("Failed to create writer with level 22: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
}