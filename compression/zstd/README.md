# Zstd Compression Package

This package provides zstd compression support with runtime detection of libzstd availability.

## Features

- **Runtime Detection**: Automatically detects and uses libzstd if available
- **Dual Implementation**: Falls back to pure Go implementation when libzstd is not available
- **High Compression Levels**: Supports compression levels up to 22 when using libzstd
- **Parallel Compression**: Automatically uses multiple CPU cores for faster compression

## Parallel Compression

The zstd compression automatically uses multiple CPU cores for faster compression:

- **Default**: Uses 75% of physical CPU cores
- **Configuration**: Set `ZSTD_WORKERS` environment variable to override

### Examples

```bash
# Use 4 compression workers
export ZSTD_WORKERS=4

# Use single-threaded compression
export ZSTD_WORKERS=1

# Use automatic detection (default)
unset ZSTD_WORKERS
```

### Performance Considerations

- **Pure Go Implementation**: Supports parallel compression with multiple workers
- **Gozstd (libzstd)**: Supports parallel compression with multiple workers
- Only one layer is compressed at a time, so resource usage is controlled

### Memory Usage

Memory usage scales with the number of workers.
For memory-constrained environments, limit workers:
```bash
export ZSTD_WORKERS=4
```

## Compression Levels

- **Pure Go**: Levels 0-11 (uses klauspost/compress)
- **libzstd**: Levels 0-22 (via gozstd wrapper)

The implementation automatically selects the best available option based on the requested compression level and library availability.

## Usage with ctr-remote

When using `ctr-remote convert` with zstd:chunked compression:

```bash
# Convert with default parallel compression
ctr-remote convert --zstdchunked --oci registry.io/image:tag registry.io/image:zstd

# Convert with custom worker count
ZSTD_WORKERS=4 ctr-remote convert --zstdchunked --oci registry.io/image:tag registry.io/image:zstd

# Convert with maximum compression (uses libzstd if available)
ctr-remote convert --zstdchunked --zstdchunked-compression-level 22 --oci registry.io/image:tag registry.io/image:zstd
```

## Testing

Run tests with specific tags:

```bash
# Basic tests
go test ./compression/zstd

# Run all zstd tests including stress tests
make test-zstd
```
