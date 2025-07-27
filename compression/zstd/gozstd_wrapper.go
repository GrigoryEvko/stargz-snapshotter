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
	"fmt"
	"io"

	"github.com/valyala/gozstd"
)

// GozstdCompressor implements Compressor using the gozstd library (CGO wrapper of libzstd)
type GozstdCompressor struct {
	available bool
}

// NewGozstdCompressor creates a new gozstd-based compressor
func NewGozstdCompressor() *GozstdCompressor {
	// Test if libzstd is actually available by trying to compress
	available := false
	defer func() {
		if r := recover(); r != nil {
			available = false
		}
	}()

	// Try a small compression to verify libzstd works
	_, err := gozstd.Compress(nil, []byte("test"))
	available = (err == nil)

	return &GozstdCompressor{available: available}
}

// NewWriter creates a new zstd writer with the specified compression level
func (g *GozstdCompressor) NewWriter(w io.Writer, level int) (WriteFlushCloser, error) {
	if !g.available {
		return nil, fmt.Errorf("libzstd not available")
	}
	
	// gozstd supports compression levels 1-22
	if level < 1 {
		level = gozstd.DefaultCompressionLevel
	} else if level > 22 {
		level = 22
	}
	
	return gozstd.NewWriterLevel(w, level), nil
}

// NewReader creates a new zstd reader
func (g *GozstdCompressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	if !g.available {
		return nil, fmt.Errorf("libzstd not available")
	}
	return gozstd.NewReader(r), nil
}

// Name returns the name of the compressor implementation
func (g *GozstdCompressor) Name() string {
	if g.available {
		return fmt.Sprintf("libzstd/%s (via gozstd)", gozstd.VersionString())
	}
	return "gozstd (unavailable)"
}

// IsLibzstdAvailable returns true if native libzstd is available
func (g *GozstdCompressor) IsLibzstdAvailable() bool {
	return g.available
}

// MaxCompressionLevel returns the maximum supported compression level
func (g *GozstdCompressor) MaxCompressionLevel() int {
	if g.available {
		return 22 // Full zstd range
	}
	return 0
}