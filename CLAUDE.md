# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make build          # Build binary to bin/imagedupfinder
make test           # Run all tests
go test -v ./internal -run TestName  # Run single test
make run ARGS="scan ./photos"        # Build and run with args
make fmt            # Format code
make lint           # Run golangci-lint
make tidy           # Run go mod tidy
```

## Architecture Overview

CLI tool for detecting duplicate/similar images using perceptual hashing, written in Go with Cobra.

### Core Flow

1. **Scan** (`cmd/scan.go`): Walks folders, hashes images in parallel, groups duplicates, stores in SQLite
2. **List** (`cmd/list.go`): Displays duplicate groups from database
3. **Clean** (`cmd/clean.go`): Removes or moves lower-quality duplicates

### Key Components

- **Matcher Interface** (`internal/grouper.go:8`): Polymorphic duplicate detection
  - `PerceptualMatcher`: Groups by Hamming distance using Union-Find algorithm
  - `ExactMatcher`: Groups by SHA256 file hash
- **Scanner** (`internal/scanner.go`): Parallel folder scanning with configurable workers via functional options pattern
- **Hasher** (`internal/hasher.go`): Computes pHash using goimagehash library, extracts EXIF, calculates quality scores
- **Storage** (`internal/storage.go`): SQLite persistence with versioned schema migrations

### Scoring System

Images are ranked by: `resolution × format_multiplier × exif_multiplier`
- Format multipliers: PNG/TIFF/BMP=1.2, WebP=1.1, JPEG=1.0, GIF=0.9
- EXIF multiplier: 1.1 if present (prefers originals over SNS-downloaded copies)

### Database Migrations

Schema uses version tracking (`schema_version` table). Add new migrations to `migrations` slice in `internal/storage.go:48`. Each migration must be idempotent.
