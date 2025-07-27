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
	"runtime"
	"strconv"

	"github.com/containerd/log"
	"github.com/shirou/gopsutil/v4/cpu"
)

// GetOptimalWorkerCount returns the optimal number of compression workers
// based on physical CPU cores. It can be overridden by the ZSTD_WORKERS
// environment variable.
func GetOptimalWorkerCount() int {
	// Check environment variable first
	if workers := os.Getenv("ZSTD_WORKERS"); workers != "" {
		if n, err := strconv.Atoi(workers); err == nil && n > 0 {
			log.L.Debugf("Using ZSTD_WORKERS=%d from environment", n)
			return n
		}
		log.L.Warnf("Invalid ZSTD_WORKERS value: %s, using automatic detection", workers)
	}
	
	// Try to get physical cores
	if cores, err := cpu.Counts(false); err == nil && cores > 0 {
		// Use 75% of physical cores to leave room for other processes
		workers := cores * 3 / 4
		if workers < 1 {
			workers = 1
		}
		log.L.Debugf("Detected %d physical cores, using %d workers for zstd compression", cores, workers)
		return workers
	}
	
	// Fallback to runtime.NumCPU() / 2 (assume hyperthreading)
	logical := runtime.NumCPU()
	workers := logical / 2
	if workers < 1 {
		workers = 1
	}
	log.L.Debugf("Could not detect physical cores, using %d workers based on %d logical CPUs", workers, logical)
	return workers
}