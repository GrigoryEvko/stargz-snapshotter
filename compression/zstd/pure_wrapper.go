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

	"github.com/klauspost/compress/zstd"
)

// PureGoCompressor implements Compressor using the pure Go klauspost/compress/zstd library
type PureGoCompressor struct{}

// NewPureGoCompressor creates a new pure Go compressor
func NewPureGoCompressor() *PureGoCompressor {
	return &PureGoCompressor{}
}

// NewWriter creates a new zstd writer with the specified compression level
func (p *PureGoCompressor) NewWriter(w io.Writer, level int) (WriteFlushCloser, error) {
	// Validate compression level
	// Pure Go implementation supports levels 0-11 (mapped from zstd levels)
	if level < 0 || level > 11 {
		return nil, fmt.Errorf("invalid compression level %d: must be between 0 and 11", level)
	}
	
	// Map the level to the klauspost/compress encoder level
	encoderLevel := zstd.EncoderLevelFromZstd(level)
	
	// Get optimal worker count for parallel compression
	workers := GetOptimalWorkerCount()
	
	enc, err := zstd.NewWriter(w, 
		zstd.WithEncoderLevel(encoderLevel),
		zstd.WithEncoderConcurrency(workers))
	if err != nil {
		return nil, err
	}
	
	return enc, nil
}

// NewReader creates a new zstd reader
func (p *PureGoCompressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	
	// Wrap the decoder to implement io.ReadCloser
	return &zstdReadCloser{dec}, nil
}

// Name returns the name of the compressor implementation
func (p *PureGoCompressor) Name() string {
	return "pure-go (klauspost/compress)"
}

// IsLibzstdAvailable returns false for pure Go implementation
func (p *PureGoCompressor) IsLibzstdAvailable() bool {
	return false
}

// MaxCompressionLevel returns the maximum supported compression level
func (p *PureGoCompressor) MaxCompressionLevel() int {
	// Pure Go implementation maps levels approximately:
	// 1 -> SpeedFastest
	// 3 -> SpeedDefault  
	// 7-8 -> SpeedBetterCompression
	// 11+ -> SpeedBestCompression
	return 11
}

// zstdReadCloser wraps zstd.Decoder to implement io.ReadCloser
type zstdReadCloser struct {
	*zstd.Decoder
}

func (z *zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}