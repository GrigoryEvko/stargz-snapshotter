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

import "io"

// WriteFlushCloser is an io.WriteCloser that also supports Flush
type WriteFlushCloser interface {
	io.WriteCloser
	Flush() error
}

// Compressor is the interface for zstd compression implementations
type Compressor interface {
	// NewWriter creates a new zstd writer with the specified compression level
	NewWriter(w io.Writer, level int) (WriteFlushCloser, error)
	
	// NewReader creates a new zstd reader
	NewReader(r io.Reader) (io.ReadCloser, error)
	
	// Name returns the name of the compressor implementation
	Name() string
	
	// IsLibzstdAvailable returns true if native libzstd is available
	IsLibzstdAvailable() bool
	
	// MaxCompressionLevel returns the maximum supported compression level
	MaxCompressionLevel() int
}