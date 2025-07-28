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
	"sync"
)

var (
	defaultCompressor Compressor
	once              sync.Once
)

// GetCompressor returns the appropriate zstd compressor based on runtime availability
func GetCompressor() Compressor {
	once.Do(func() {
		// Check if user wants to force pure Go implementation
		if os.Getenv("STARGZ_FORCE_PURE_GO_ZSTD") == "1" {
			defaultCompressor = NewPureGoCompressor()
			return
		}

		// Try gozstd first
		gozstd := NewGozstdCompressor()
		if gozstd.IsLibzstdAvailable() {
			defaultCompressor = gozstd
		} else {
			defaultCompressor = NewPureGoCompressor()
		}
	})
	return defaultCompressor
}

// SetCompressor allows overriding the default compressor (mainly for testing)
func SetCompressor(c Compressor) {
	defaultCompressor = c
}