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

//go:build zstd_unit || zstd_all

package testsuite

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


// TestBasicCompressDecompress tests basic compress/decompress operations
func TestBasicCompressDecompress(t *testing.T) {
	suite := NewTestSuite()

	for _, impl := range suite.implementations {
		if impl.Skip {
			t.Logf("Skipping %s: %s", impl.Name, impl.SkipReason)
			continue
		}

		t.Run(impl.Name, func(t *testing.T) {
			for _, pattern := range TestDataPatterns {
				t.Run(pattern.Name, func(t *testing.T) {
					for _, size := range pattern.Sizes {
						t.Run(formatSize(size), func(t *testing.T) {
							testData := pattern.Generator(size)
							
							// Test at different compression levels
							levels := []int{1, 3, 11}
							if impl.Compressor.MaxCompressionLevel() >= 22 {
								levels = append(levels, 22)
							}

							for _, level := range levels {
								t.Run(formatLevel(level), func(t *testing.T) {
									// Compress
									var compressed bytes.Buffer
									writer, err := impl.Compressor.NewWriter(&compressed, level)
									require.NoError(t, err)

									n, err := writer.Write(testData)
									require.NoError(t, err)
									assert.Equal(t, len(testData), n)

									err = writer.Close()
									require.NoError(t, err)

									compressedData := compressed.Bytes()
									if size > 0 {
										assert.NotEmpty(t, compressedData, "Compressed data should not be empty")
									}

									// Decompress
									reader, err := impl.Compressor.NewReader(bytes.NewReader(compressedData))
									require.NoError(t, err)
									defer reader.Close()

									decompressed, err := io.ReadAll(reader)
									require.NoError(t, err)

									// Verify
									assert.Equal(t, testData, decompressed, "Decompressed data should match original")
									
									// Log compression ratio
									if size > 0 {
										ratio := float64(len(compressedData)) * 100 / float64(size)
										t.Logf("Compression ratio: %.2f%% (%d -> %d bytes)", 
											ratio, size, len(compressedData))
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

// TestEdgeCases tests edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	suite := NewTestSuite()

	for _, impl := range suite.implementations {
		if impl.Skip {
			continue
		}

		t.Run(impl.Name, func(t *testing.T) {
			t.Run("EmptyData", func(t *testing.T) {
				var compressed bytes.Buffer
				writer, err := impl.Compressor.NewWriter(&compressed, 3)
				require.NoError(t, err)
				
				err = writer.Close()
				require.NoError(t, err)

				reader, err := impl.Compressor.NewReader(&compressed)
				require.NoError(t, err)
				defer reader.Close()

				data, err := io.ReadAll(reader)
				require.NoError(t, err)
				assert.Empty(t, data)
			})

			t.Run("InvalidCompressionLevel", func(t *testing.T) {
				// Test negative level
				_, err := impl.Compressor.NewWriter(io.Discard, -1)
				assert.Error(t, err)

				// Test excessive level
				_, err = impl.Compressor.NewWriter(io.Discard, 100)
				assert.Error(t, err)
			})

			t.Run("CorruptedData", func(t *testing.T) {
				// Create some corrupted data
				corruptedData := []byte{0xFF, 0xFE, 0xFD, 0xFC}
				
				reader, err := impl.Compressor.NewReader(bytes.NewReader(corruptedData))
				if err == nil {
					_, err = io.ReadAll(reader)
					reader.Close()
				}
				assert.Error(t, err, "Should fail on corrupted data")
			})

			t.Run("MultipleWrites", func(t *testing.T) {
				var compressed bytes.Buffer
				writer, err := impl.Compressor.NewWriter(&compressed, 3)
				require.NoError(t, err)

				// Write multiple chunks
				chunks := [][]byte{
					[]byte("First chunk"),
					[]byte("Second chunk"),
					[]byte("Third chunk"),
				}

				for _, chunk := range chunks {
					n, err := writer.Write(chunk)
					require.NoError(t, err)
					assert.Equal(t, len(chunk), n)
				}

				err = writer.Close()
				require.NoError(t, err)

				// Decompress and verify
				reader, err := impl.Compressor.NewReader(&compressed)
				require.NoError(t, err)
				defer reader.Close()

				decompressed, err := io.ReadAll(reader)
				require.NoError(t, err)

				expected := bytes.Join(chunks, nil)
				assert.Equal(t, expected, decompressed)
			})
		})
	}
}

// TestFlushBehavior tests the Flush() method behavior
func TestFlushBehavior(t *testing.T) {
	suite := NewTestSuite()

	for _, impl := range suite.implementations {
		if impl.Skip {
			continue
		}

		t.Run(impl.Name, func(t *testing.T) {
			var buf bytes.Buffer
			writer, err := impl.Compressor.NewWriter(&buf, 3)
			require.NoError(t, err)

			// Write and flush multiple times
			chunks := []string{"chunk1", "chunk2", "chunk3"}
			for i, chunk := range chunks {
				_, err := writer.Write([]byte(chunk))
				require.NoError(t, err)

				err = writer.Flush()
				require.NoError(t, err)

				// After flush, we should have some data
				flushedSize := buf.Len()
				assert.Greater(t, flushedSize, 0, "Should have data after flush %d", i+1)
			}

			err = writer.Close()
			require.NoError(t, err)

			// Verify we can decompress the complete stream
			reader, err := impl.Compressor.NewReader(&buf)
			require.NoError(t, err)
			defer reader.Close()

			decompressed, err := io.ReadAll(reader)
			require.NoError(t, err)

			expected := strings.Join(chunks, "")
			assert.Equal(t, expected, string(decompressed))
		})
	}
}

