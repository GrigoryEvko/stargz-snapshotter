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
	"os"
	"testing"

	"github.com/containerd/stargz-snapshotter/compression/zstd"
)

// TestMain sets up single-threaded compression for all tests in this package
// to ensure deterministic behavior and consistent test results.
func TestMain(m *testing.M) {
	// Set single-threaded mode for all tests unless explicitly overridden
	if os.Getenv("ZSTD_WORKERS") == "" {
		os.Setenv("ZSTD_WORKERS", "1")
	}

	// Run tests
	code := m.Run()
	
	os.Exit(code)
}

// SetupTest is a helper that can be used in individual tests that need
// specific worker configuration.
func SetupTest(t *testing.T) func() {
	return zstd.SetupSingleThreadedTest(t)
}