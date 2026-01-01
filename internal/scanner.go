package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Scanner scans folders for images and computes hashes
type Scanner struct {
	hasher     *Hasher
	workers    int
	timeout    time.Duration
	progressFn func(scanned, total int, current string)
}

// ScannerOption configures a Scanner
type ScannerOption func(*Scanner)

// WithWorkers sets the number of parallel workers
func WithWorkers(n int) ScannerOption {
	return func(s *Scanner) {
		if n > 0 {
			s.workers = n
		}
	}
}

// WithTimeout sets the timeout for hashing each image
func WithTimeout(d time.Duration) ScannerOption {
	return func(s *Scanner) {
		s.timeout = d
	}
}

// WithProgress sets a progress callback
func WithProgress(fn func(scanned, total int, current string)) ScannerOption {
	return func(s *Scanner) {
		s.progressFn = fn
	}
}

// NewScanner creates a new Scanner
func NewScanner(opts ...ScannerOption) *Scanner {
	s := &Scanner{
		hasher:  NewHasher(),
		workers: 8,
		timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ScanFolder scans a folder for images and returns their info
func (s *Scanner) ScanFolder(folder string) ([]*ImageInfo, error) {
	// First, collect all image paths
	var paths []string
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}
		if IsSupportedImage(path) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk folder: %w", err)
	}

	if len(paths) == 0 {
		return nil, nil
	}

	// Process images in parallel
	var (
		results   []*ImageInfo
		resultsMu sync.Mutex
		wg        sync.WaitGroup
		scanned   int64
		total     = len(paths)
	)

	// Create work channel
	work := make(chan string, len(paths))
	for _, p := range paths {
		work <- p
	}
	close(work)

	// Start workers
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range work {
				info, err := s.hasher.HashImageWithTimeout(path, s.timeout)
				if err != nil {
					// Skip failed images silently
					atomic.AddInt64(&scanned, 1)
					continue
				}

				resultsMu.Lock()
				results = append(results, info)
				resultsMu.Unlock()

				n := atomic.AddInt64(&scanned, 1)
				if s.progressFn != nil {
					s.progressFn(int(n), total, path)
				}
			}
		}()
	}

	wg.Wait()

	return results, nil
}

// ScanFolders scans multiple folders
func (s *Scanner) ScanFolders(folders []string) ([]*ImageInfo, error) {
	var allResults []*ImageInfo
	for _, folder := range folders {
		results, err := s.ScanFolder(folder)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, results...)
	}
	return allResults, nil
}
