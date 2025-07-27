# ZSTD Compression Test Suite

This test suite provides comprehensive testing for the zstd compression implementation with runtime detection of libzstd.

## Test Categories

### Unit Tests (`zstd_unit`)
Basic correctness tests for compression and decompression operations.

- **TestBasicCompressDecompress**: Tests basic compress/decompress operations with various data patterns and sizes
- **TestEdgeCases**: Tests edge cases like empty data, invalid compression levels, corrupted data
- **TestFlushBehavior**: Tests the Flush() method behavior for streaming scenarios

### Integration Tests (`zstd_integration`)
Tests that verify cross-implementation compatibility and integration scenarios.

- **TestCrossImplementationCompatibility**: Ensures data compressed by one implementation can be decompressed by another
- **TestCompressionLevelCompatibility**: Verifies different compression levels produce compatible output
- **TestStreamingCompatibility**: Tests streaming compression/decompression across implementations

### Performance Benchmarks (`zstd_benchmark`)
Benchmarks for measuring compression and decompression performance.

- **BenchmarkCompression**: Measures compression performance at various levels and data sizes
- **BenchmarkDecompression**: Measures decompression performance
- **BenchmarkMemoryUsage**: Tracks memory allocations during operations
- **BenchmarkParallelOperations**: Tests parallel compression/decompression performance
- **TestCompressionRatios**: Analyzes compression ratios for different data types
- **TestThroughput**: Measures compression/decompression throughput in MB/s

### Stress Tests (`zstd_stress`)
Stress tests for reliability and resource usage under load.

- **TestConcurrentOperations**: Tests concurrent compression/decompression with multiple goroutines
- **TestMemoryStress**: Tests memory usage under stress conditions
- **TestLargeFileStress**: Tests handling of very large files (100MB+)
- **TestRapidWriterCreation**: Tests rapid creation and destruction of writers
- **TestErrorRecovery**: Tests error recovery scenarios

## Running Tests

### Using Make targets

From the stargz-snapshotter repository root:

```bash
# Run all zstd tests (unit + integration)
make test-zstd

# Run only unit tests
make test-zstd-unit

# Run only integration tests  
make test-zstd-integration

# Run performance benchmarks
make test-zstd-benchmark

# Run stress tests (may take longer)
make test-zstd-stress

# Run everything including benchmarks and stress tests
make test-zstd-all
```

### Using go test directly

```bash
# Run unit tests
go test -v ./compression/zstd/testsuite/... -tags zstd_unit

# Run integration tests
go test -v ./compression/zstd/testsuite/... -tags zstd_integration

# Run benchmarks
go test -v ./compression/zstd/testsuite/... -tags zstd_benchmark -bench=. -benchmem

# Run stress tests with extended timeout
go test -v ./compression/zstd/testsuite/... -tags zstd_stress -timeout 30m

# Run all tests
go test -v ./compression/zstd/testsuite/... -tags zstd_all -bench=. -benchmem -timeout 30m
```

### Running specific tests

```bash
# Run a specific test function
go test -v ./compression/zstd/testsuite/... -tags zstd_unit -run TestBasicCompressDecompress

# Run a specific benchmark
go test -v ./compression/zstd/testsuite/... -tags zstd_benchmark -bench BenchmarkCompression -benchmem
```

## Test Data Patterns

The test suite uses various data patterns to ensure comprehensive coverage:

1. **Random**: Incompressible random data
2. **Zeros**: Highly compressible zero-filled data
3. **Repetitive**: Text patterns with high compressibility
4. **Binary**: Sequential binary patterns
5. **Text**: Natural language text
6. **JSON**: Structured JSON data

## Build Tags

- `zstd_unit`: Enables unit tests
- `zstd_integration`: Enables integration tests
- `zstd_benchmark`: Enables performance benchmarks
- `zstd_stress`: Enables stress tests
- `zstd_all`: Enables all tests

## Environment Variables

- `ZSTD_FORCE_IMPLEMENTATION`: Force a specific implementation (`klauspost` or `gozstd`)

## Test Requirements

- Go 1.19 or later
- CGO enabled for libzstd tests (gozstd implementation)
- Sufficient memory for stress tests (at least 2GB recommended)

## Interpreting Results

### Compression Ratios
Lower percentages indicate better compression. Typical ratios:
- Random data: ~100% (no compression)
- Text data: 20-40%
- JSON data: 15-30%
- Zeros: <1%

### Throughput
Measured in MB/s. Higher is better. Expected ranges:
- Pure Go: 100-500 MB/s depending on CPU
- libzstd: 200-1000 MB/s depending on CPU and compression level

### Memory Usage
The benchmarks report allocations per operation. Lower allocation counts and bytes indicate better memory efficiency.