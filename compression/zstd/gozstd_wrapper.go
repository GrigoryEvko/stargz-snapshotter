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

/*
#cgo LDFLAGS: -lzstd
#include <zstd.h>

// Define ZSTD_c_nbWorkers if not available (for older zstd versions)
#ifndef ZSTD_c_nbWorkers
#define ZSTD_c_nbWorkers 400
#endif
*/
import "C"

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"

	"github.com/containerd/log"
	"github.com/valyala/gozstd"
)

// GozstdCompressor implements Compressor using the gozstd library (CGO wrapper of libzstd)
type GozstdCompressor struct {
	available bool
}

// gozstdWriterWrapper wraps gozstd.Writer and sets multi-threading
type gozstdWriterWrapper struct {
	*gozstd.Writer
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
	compressed := gozstd.Compress(nil, []byte("test"))
	available = (compressed != nil)

	return &GozstdCompressor{available: available}
}

// NewWriter creates a new zstd writer with the specified compression level
func (g *GozstdCompressor) NewWriter(w io.Writer, level int) (WriteFlushCloser, error) {
	if !g.available {
		return nil, fmt.Errorf("libzstd not available")
	}
	
	// Validate compression level
	if level < 0 || level > 22 {
		return nil, fmt.Errorf("invalid compression level %d: must be between 0 and 22", level)
	}
	
	// Use default level if 0 is specified
	if level == 0 {
		level = gozstd.DefaultCompressionLevel
	}
	
	// Create the writer
	writer := gozstd.NewWriterLevel(w, level)
	
	// Get optimal worker count
	workers := GetOptimalWorkerCount()
	
	// Use reflection to access the private cs field (ZSTD_CStream pointer)
	writerValue := reflect.ValueOf(writer).Elem()
	csField := writerValue.FieldByName("cs")
	
	if csField.IsValid() && csField.CanAddr() {
		// Get the pointer to ZSTD_CStream
		csPtr := (*C.ZSTD_CStream)(unsafe.Pointer(csField.UnsafeAddr()))
		
		// Set the number of workers
		result := C.ZSTD_CCtx_setParameter(
			(*C.ZSTD_CCtx)(csPtr),
			C.ZSTD_c_nbWorkers,
			C.int(workers))
		
		if C.ZSTD_isError(result) != 0 {
			// Log the error but continue - multi-threading might not be available
			log.L.Debugf("Failed to set zstd workers to %d, using single-threaded mode", workers)
		} else {
			log.L.Debugf("Successfully set zstd workers to %d", workers)
		}
	} else {
		log.L.Debug("Unable to access gozstd internal structure for multi-threading configuration")
	}
	
	return &gozstdWriterWrapper{writer}, nil
}

// NewReader creates a new zstd reader
func (g *GozstdCompressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	if !g.available {
		return nil, fmt.Errorf("libzstd not available")
	}
	reader := gozstd.NewReader(r)
	return &gozstdReaderWrapper{reader}, nil
}

// gozstdReaderWrapper wraps gozstd.Reader to implement io.ReadCloser
type gozstdReaderWrapper struct {
	*gozstd.Reader
}

func (w *gozstdReaderWrapper) Close() error {
	w.Reader.Release()
	return nil
}

// Name returns the name of the compressor implementation
func (g *GozstdCompressor) Name() string {
	if g.available {
		return "libzstd (via gozstd)"
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