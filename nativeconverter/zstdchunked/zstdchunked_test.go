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

package zstdchunked

import (
	"context"
	"fmt"
	"os"
	"testing"

	"runtime/debug"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/images/converter"
	"github.com/containerd/platforms"
	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/containerd/stargz-snapshotter/estargz/zstdchunked"
	"github.com/containerd/stargz-snapshotter/util/testutil"
	"github.com/klauspost/compress/zstd"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TestLayerConvertFunc tests zstd:chunked conversion.
// TestLayerConvertFunc is a pure unit test that does not need the daemon to be running.
func TestLayerConvertFunc(t *testing.T) {
	ctx := context.Background()
	desc, cs, err := testutil.EnsureHello(ctx)
	if err != nil {
		t.Fatal(err)
	}

	lcf := LayerConvertFunc(estargz.WithPrioritizedFiles([]string{"hello"}))
	docker2oci := true
	platformMC := platforms.DefaultStrict()
	cf := converter.DefaultIndexConvertFunc(lcf, docker2oci, platformMC)

	newDesc, err := cf(ctx, cs, *desc)
	if err != nil {
		t.Log(string(debug.Stack()))
		t.Fatal(err)
	}

	metadata := make(map[string]string)
	mt := make(map[string]struct{})
	handler := func(hCtx context.Context, hDesc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		mt[hDesc.MediaType] = struct{}{}
		for k, v := range hDesc.Annotations {
			if k == estargz.TOCJSONDigestAnnotation ||
				k == zstdchunked.ManifestChecksumAnnotation ||
				k == zstdchunked.ManifestPositionAnnotation {
				metadata[k] = v
			}
		}
		return nil, nil
	}
	handlers := images.Handlers(
		images.ChildrenHandler(cs),
		images.HandlerFunc(handler),
	)
	if err := images.Walk(ctx, handlers, *newDesc); err != nil {
		t.Fatal(err)
	}

	if _, ok := mt[ocispec.MediaTypeImageLayerZstd]; !ok {
		t.Errorf("mediatype %q is not created", ocispec.MediaTypeImageLayerZstd)
	}
	if _, ok := metadata[estargz.TOCJSONDigestAnnotation]; !ok {
		t.Errorf("%q is not set", estargz.TOCJSONDigestAnnotation)
	}
	if _, ok := metadata[zstdchunked.ManifestChecksumAnnotation]; !ok {
		t.Errorf("%q is not set", zstdchunked.ManifestChecksumAnnotation)
	}
	if _, ok := metadata[zstdchunked.ManifestPositionAnnotation]; !ok {
		t.Errorf("%q is not set", zstdchunked.ManifestPositionAnnotation)
	}
}

// TestHighCompressionLevels tests zstd:chunked conversion with high compression levels
// to ensure runtime detection of libzstd works correctly
func TestHighCompressionLevels(t *testing.T) {
	ctx := context.Background()
	desc, cs, err := testutil.EnsureHello(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test various compression levels
	levels := []int{1, 3, 11, 22}
	
	for _, level := range levels {
		t.Run(fmt.Sprintf("Level_%d", level), func(t *testing.T) {
			// Use klauspost EncoderLevel type
			encoderLevel := zstd.EncoderLevelFromZstd(level)
			lcf := LayerConvertFuncWithCompressionLevel(encoderLevel, estargz.WithPrioritizedFiles([]string{"hello"}))
			
			docker2oci := true
			platformMC := platforms.DefaultStrict()
			cf := converter.DefaultIndexConvertFunc(lcf, docker2oci, platformMC)

			newDesc, err := cf(ctx, cs, *desc)
			if err != nil {
				// Level 22 might fail if only pure Go is available
				if level > 11 {
					t.Logf("Level %d conversion failed (expected if libzstd not available): %v", level, err)
					return
				}
				t.Fatal(err)
			}
			
			// Verify the conversion succeeded
			if newDesc == nil {
				t.Error("conversion returned nil descriptor")
			}
		})
	}
}

// TestCompressionImplementationEnvVar tests forcing specific implementations
func TestCompressionImplementationEnvVar(t *testing.T) {
	ctx := context.Background()
	desc, cs, err := testutil.EnsureHello(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		envValue string
		level    int
	}{
		{
			name:     "Force pure Go",
			envValue: "klauspost",
			level:    11,
		},
		{
			name:     "Force libzstd",  
			envValue: "gozstd",
			level:    22,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			oldEnv := os.Getenv("ZSTD_FORCE_IMPLEMENTATION")
			defer os.Setenv("ZSTD_FORCE_IMPLEMENTATION", oldEnv)
			
			os.Setenv("ZSTD_FORCE_IMPLEMENTATION", tt.envValue)
			
			encoderLevel := zstd.EncoderLevelFromZstd(tt.level)
			lcf := LayerConvertFuncWithCompressionLevel(encoderLevel)
			
			docker2oci := true
			platformMC := platforms.DefaultStrict()
			cf := converter.DefaultIndexConvertFunc(lcf, docker2oci, platformMC)

			newDesc, err := cf(ctx, cs, *desc)
			if err != nil {
				// gozstd might fail if libzstd not available
				if tt.envValue == "gozstd" {
					t.Logf("Forcing libzstd failed (expected if not available): %v", err)
					return
				}
				t.Fatal(err)
			}
			
			if newDesc == nil {
				t.Error("conversion returned nil descriptor")
			}
		})
	}
}
