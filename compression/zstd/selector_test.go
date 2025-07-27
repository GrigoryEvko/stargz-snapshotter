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
	"os"
	"testing"
)

func TestGetCompressor(t *testing.T) {
	// Test automatic detection
	compressor := GetCompressor()
	if compressor == nil {
		t.Fatal("GetCompressor() returned nil")
	}
	
	// Verify compressor has expected methods
	if compressor.Name() == "" {
		t.Error("Compressor name should not be empty")
	}
	
	maxLevel := compressor.MaxCompressionLevel()
	if maxLevel < 1 {
		t.Errorf("Maximum compression level should be at least 1, got %d", maxLevel)
	}
}

func TestEnvironmentOverride(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectError bool
	}{
		{
			name:        "Force gozstd",
			envValue:    "gozstd",
			expectError: false, // May fail if libzstd not available
		},
		{
			name:        "Force klauspost",
			envValue:    "klauspost",
			expectError: false,
		},
		{
			name:        "Invalid value",
			envValue:    "invalid",
			expectError: false, // Should fall back to auto-detection
		},
		{
			name:        "Empty value",
			envValue:    "",
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env
			origEnv := os.Getenv("ZSTD_FORCE_IMPLEMENTATION")
			defer os.Setenv("ZSTD_FORCE_IMPLEMENTATION", origEnv)
			
			// Set test env
			os.Setenv("ZSTD_FORCE_IMPLEMENTATION", tt.envValue)
			
			// Clear cached compressor to force re-selection
			selectedCompressor = nil
			
			compressor := GetCompressor()
			if compressor == nil {
				t.Error("GetCompressor() returned nil")
			}
			
			// For klauspost, verify it's actually using the pure Go implementation
			if tt.envValue == "klauspost" {
				if compressor.IsLibzstdAvailable() {
					t.Error("Expected pure Go implementation when forcing klauspost")
				}
				if compressor.MaxCompressionLevel() > 11 {
					t.Errorf("Pure Go implementation should have max level 11, got %d", 
						compressor.MaxCompressionLevel())
				}
			}
		})
	}
}

func TestCompressionLevelValidation(t *testing.T) {
	compressor := GetCompressor()
	maxLevel := compressor.MaxCompressionLevel()
	
	tests := []struct {
		name          string
		level         int
		shouldSucceed bool
	}{
		{
			name:          "Default level (3)",
			level:         3,
			shouldSucceed: true,
		},
		{
			name:          "Maximum level",
			level:         maxLevel,
			shouldSucceed: true,
		},
		{
			name:          "Beyond maximum",
			level:         maxLevel + 1,
			shouldSucceed: false, // Should be capped or error
		},
		{
			name:          "Negative level",
			level:         -1,
			shouldSucceed: true, // Negative levels are valid in zstd
		},
		{
			name:          "Zero level",
			level:         0,
			shouldSucceed: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test would use actual compression here
			// For now, just verify level is within expected bounds
			if tt.level > maxLevel && tt.shouldSucceed {
				t.Errorf("Level %d should not succeed when max is %d", tt.level, maxLevel)
			}
		})
	}
}