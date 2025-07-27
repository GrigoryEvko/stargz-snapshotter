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

package testsuite

import (
	"crypto/rand"
	"fmt"

	"github.com/containerd/stargz-snapshotter/compression/zstd"
)

// TestSuite provides a comprehensive test suite for zstd compression
type TestSuite struct {
	implementations []Implementation
}

// Implementation represents a zstd implementation to test
type Implementation struct {
	Name       string
	Compressor zstd.Compressor
	Skip       bool
	SkipReason string
}

// NewTestSuite creates a new test suite with all available implementations
func NewTestSuite() *TestSuite {
	suite := &TestSuite{
		implementations: []Implementation{
			{
				Name:       "PureGo",
				Compressor: zstd.NewPureGoCompressor(),
			},
			{
				Name:       "Gozstd",
				Compressor: zstd.NewGozstdCompressor(),
			},
		},
	}

	// Check if gozstd is actually available
	if !suite.implementations[1].Compressor.IsLibzstdAvailable() {
		suite.implementations[1].Skip = true
		suite.implementations[1].SkipReason = "libzstd not available"
	}

	return suite
}

// TestDataPatterns defines various data patterns for testing
var TestDataPatterns = []struct {
	Name        string
	Generator   func(size int) []byte
	Sizes       []int
	Description string
}{
	{
		Name: "Random",
		Generator: func(size int) []byte {
			data := make([]byte, size)
			rand.Read(data)
			return data
		},
		Sizes:       []int{0, 1, 100, 1024, 65536, 1048576},
		Description: "Random incompressible data",
	},
	{
		Name: "Zeros",
		Generator: func(size int) []byte {
			return make([]byte, size)
		},
		Sizes:       []int{0, 1, 100, 1024, 65536, 1048576},
		Description: "Highly compressible zeros",
	},
	{
		Name: "Repetitive",
		Generator: func(size int) []byte {
			pattern := []byte("The quick brown fox jumps over the lazy dog. ")
			data := make([]byte, size)
			for i := 0; i < size; i++ {
				data[i] = pattern[i%len(pattern)]
			}
			return data
		},
		Sizes:       []int{100, 1024, 65536, 1048576},
		Description: "Repetitive text pattern",
	},
	{
		Name: "Binary",
		Generator: func(size int) []byte {
			data := make([]byte, size)
			for i := 0; i < size; i++ {
				data[i] = byte(i & 0xFF)
			}
			return data
		},
		Sizes:       []int{256, 1024, 65536},
		Description: "Binary sequence pattern",
	},
}

// Helper functions
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

func formatLevel(level int) string {
	return fmt.Sprintf("Level%d", level)
}