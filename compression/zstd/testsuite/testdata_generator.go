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
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	testDataCache     map[string][]byte
	testDataCacheMu   sync.RWMutex
	testDataInitOnce  sync.Once
)

// initTestData initializes the test data cache with real source code
func initTestData() {
	testDataCacheMu.Lock()
	defer testDataCacheMu.Unlock()
	
	testDataCache = make(map[string][]byte)
	
	// Collect Go source files from the repository
	var sourceFiles [][]byte
	
	// Walk up to find the repository root
	repoRoots := []string{
		"../../../../..", // From testsuite to nerdctl root
		"../../../../../containerd/stargz-snapshotter", // stargz-snapshotter
	}
	
	for _, root := range repoRoots {
		if _, err := os.Stat(root); err == nil {
			collectGoFiles(root, &sourceFiles)
		}
	}
	
	// If we didn't find enough files, use some embedded Go code
	if len(sourceFiles) == 0 {
		sourceFiles = append(sourceFiles, []byte(embeddedGoCode1))
		sourceFiles = append(sourceFiles, []byte(embeddedGoCode2))
		sourceFiles = append(sourceFiles, []byte(embeddedGoCode3))
	}
	
	// Create different sized test data by concatenating source files
	testDataCache["small"] = createTestData(sourceFiles, 1024)           // 1KB
	testDataCache["medium"] = createTestData(sourceFiles, 10*1024)       // 10KB
	testDataCache["large"] = createTestData(sourceFiles, 100*1024)       // 100KB
	testDataCache["xlarge"] = createTestData(sourceFiles, 1024*1024)     // 1MB
	testDataCache["xxlarge"] = createTestData(sourceFiles, 10*1024*1024) // 10MB
}

// collectGoFiles walks directory and collects Go source files
func collectGoFiles(root string, files *[][]byte) {
	maxFiles := 100 // Limit to avoid excessive memory usage
	count := 0
	
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || count >= maxFiles {
			return nil
		}
		
		// Skip vendor, .git, and test directories
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Only collect .go files
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 || len(data) > 1024*1024 { // Skip files > 1MB
			return nil
		}
		
		*files = append(*files, data)
		count++
		return nil
	})
}

// createTestData creates test data of specified size by concatenating source files
func createTestData(sources [][]byte, targetSize int) []byte {
	if len(sources) == 0 {
		// Fallback to repetitive pattern
		pattern := []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n")
		return bytes.Repeat(pattern, targetSize/len(pattern)+1)[:targetSize]
	}
	
	var buf bytes.Buffer
	buf.Grow(targetSize)
	
	// Use a deterministic random source for reproducibility
	r := rand.New(rand.NewSource(42))
	
	for buf.Len() < targetSize {
		// Pick a random source file
		source := sources[r.Intn(len(sources))]
		
		// Write the source or part of it
		remaining := targetSize - buf.Len()
		if len(source) <= remaining {
			buf.Write(source)
			buf.WriteByte('\n')
		} else {
			buf.Write(source[:remaining])
		}
	}
	
	return buf.Bytes()[:targetSize]
}

// GetSourceCodeData returns real source code of the specified size
func GetSourceCodeData(size int) []byte {
	testDataInitOnce.Do(initTestData)
	
	testDataCacheMu.RLock()
	defer testDataCacheMu.RUnlock()
	
	// Find the closest cached size
	switch {
	case size <= 1024:
		return testDataCache["small"][:size]
	case size <= 10*1024:
		return testDataCache["medium"][:size]
	case size <= 100*1024:
		return testDataCache["large"][:size]
	case size <= 1024*1024:
		return testDataCache["xlarge"][:size]
	case size <= 10*1024*1024:
		return testDataCache["xxlarge"][:size]
	default:
		// For very large sizes, repeat the largest cached data
		data := testDataCache["xxlarge"]
		result := make([]byte, size)
		for i := 0; i < size; i += len(data) {
			copy(result[i:], data)
		}
		return result
	}
}

// GetVariedSourceData returns source code with some variation for stress testing
func GetVariedSourceData(size int, seed int) []byte {
	baseData := GetSourceCodeData(size)
	
	// Create variation by inserting comments and whitespace
	r := rand.New(rand.NewSource(int64(seed)))
	result := make([]byte, size)
	
	comments := []string{
		"// TODO: optimize this\n",
		"// FIXME: handle edge case\n", 
		"// NOTE: performance critical section\n",
		"/* Multi-line comment\n * explaining the logic\n */\n",
	}
	
	pos := 0
	for pos < size {
		if r.Float32() < 0.1 && pos+100 < size { // 10% chance to insert comment
			comment := comments[r.Intn(len(comments))]
			copy(result[pos:], comment)
			pos += len(comment)
		} else {
			// Copy from base data
			remaining := size - pos
			basePos := pos % len(baseData)
			toCopy := remaining
			if basePos+toCopy > len(baseData) {
				toCopy = len(baseData) - basePos
			}
			copy(result[pos:], baseData[basePos:basePos+toCopy])
			pos += toCopy
		}
	}
	
	return result
}

// Embedded Go code samples for fallback
const embeddedGoCode1 = `package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// Compressor represents a compression algorithm
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Name() string
}

// GzipCompressor implements Compressor using gzip
type GzipCompressor struct {
	level int
}

// NewGzipCompressor creates a new gzip compressor
func NewGzipCompressor(level int) *GzipCompressor {
	return &GzipCompressor{level: level}
}

// Compress compresses data using gzip
func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, g.level)
	if err != nil {
		return nil, fmt.Errorf("creating gzip writer: %w", err)
	}
	
	_, err = w.Write(data)
	if err != nil {
		return nil, fmt.Errorf("writing to gzip: %w", err)
	}
	
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}
	
	return buf.Bytes(), nil
}

// Decompress decompresses gzip data
func (g *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer r.Close()
	
	return io.ReadAll(r)
}

// Name returns the compressor name
func (g *GzipCompressor) Name() string {
	return fmt.Sprintf("gzip-level-%d", g.level)
}
`

const embeddedGoCode2 = `package container

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Container represents a running container
type Container struct {
	ID        string            
	Name      string            
	Image     string            
	Status    ContainerStatus   
	CreatedAt time.Time         
	StartedAt *time.Time        
	Labels    map[string]string 
	mu        sync.RWMutex      
}

// ContainerStatus represents the container state
type ContainerStatus string

const (
	StatusCreated ContainerStatus = "created"
	StatusRunning ContainerStatus = "running"
	StatusStopped ContainerStatus = "stopped"
	StatusPaused  ContainerStatus = "paused"
)

// NewContainer creates a new container
func NewContainer(id, name, image string) *Container {
	return &Container{
		ID:        id,
		Name:      name,
		Image:     image,
		Status:    StatusCreated,
		CreatedAt: time.Now(),
		Labels:    make(map[string]string),
	}
}

// Start starts the container
func (c *Container) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.Status != StatusCreated && c.Status != StatusStopped {
		return fmt.Errorf("container %s is not in a startable state: %s", c.ID, c.Status)
	}
	
	// Simulate container startup
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		now := time.Now()
		c.StartedAt = &now
		c.Status = StatusRunning
		return nil
	}
}

// Stop stops the container
func (c *Container) Stop(ctx context.Context, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.Status != StatusRunning && c.Status != StatusPaused {
		return fmt.Errorf("container %s is not running: %s", c.ID, c.Status)
	}
	
	// Simulate graceful shutdown
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		c.Status = StatusStopped
		return nil
	}
}

// MarshalJSON implements json.Marshaler
func (c *Container) MarshalJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return json.Marshal(struct {
		ID        string            
		Name      string            
		Image     string            
		Status    ContainerStatus   
		CreatedAt time.Time         
		StartedAt *time.Time        
		Labels    map[string]string 
	}{
		ID:        c.ID,
		Name:      c.Name,
		Image:     c.Image,
		Status:    c.Status,
		CreatedAt: c.CreatedAt,
		StartedAt: c.StartedAt,
		Labels:    c.Labels,
	})
}
`

const embeddedGoCode3 = `package network

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
)

// IPPool manages IP address allocation
type IPPool struct {
	network   *net.IPNet
	start     uint32
	end       uint32
	allocated map[uint32]bool
	mu        sync.Mutex
}

// NewIPPool creates a new IP address pool
func NewIPPool(cidr string) (*IPPool, error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("parsing CIDR: %w", err)
	}
	
	ones, bits := network.Mask.Size()
	if bits-ones < 2 {
		return nil, errors.New("network too small")
	}
	
	start := binary.BigEndian.Uint32(network.IP.To4()) + 1 // Skip network address
	end := start + (1 << uint(bits-ones)) - 3              // Skip broadcast
	
	return &IPPool{
		network:   network,
		start:     start,
		end:       end,
		allocated: make(map[uint32]bool),
	}, nil
}

// Allocate allocates a new IP address
func (p *IPPool) Allocate() (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for i := p.start; i <= p.end; i++ {
		if !p.allocated[i] {
			p.allocated[i] = true
			ip := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip, i)
			return ip, nil
		}
	}
	
	return nil, errors.New("no available IP addresses")
}

// Release releases an allocated IP address
func (p *IPPool) Release(ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	ip4 := ip.To4()
	if ip4 == nil {
		return errors.New("not an IPv4 address")
	}
	
	if !p.network.Contains(ip) {
		return errors.New("IP not in pool network")
	}
	
	addr := binary.BigEndian.Uint32(ip4)
	if !p.allocated[addr] {
		return errors.New("IP not allocated")
	}
	
	delete(p.allocated, addr)
	return nil
}

// Stats returns pool statistics
func (p *IPPool) Stats() (total, used, available int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	total = int(p.end - p.start + 1)
	used = len(p.allocated)
	available = total - used
	return
}
`