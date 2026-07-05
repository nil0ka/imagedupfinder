package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"imagedupfinder/internal/hash"
	"imagedupfinder/internal/match"
	"imagedupfinder/internal/models"
	"imagedupfinder/internal/scan"
	"imagedupfinder/internal/storage"
)

var (
	exactMode  bool
	fullRescan bool
)

var scanCmd = &cobra.Command{
	Use:   "scan <folder>",
	Short: "Scan a folder for duplicate images",
	Long: `Scan a folder recursively for images and detect duplicates.

The scan will:
1. Find all supported images (jpg, png, gif, webp, etc.)
2. Compute perceptual hashes for each image (or file hashes with --exact)
3. Group similar images based on hash distance (or exact match with --exact)
4. Store results in the database for later use

Files already in the database whose size and modification time are unchanged
are not re-hashed, so re-scanning a large folder is fast. Use --full to force
re-hashing everything. Database entries for files that no longer exist under
the scanned folder are removed automatically.

Example:
  imagedupfinder scan ./photos
  imagedupfinder scan /path/to/images --threshold 5
  imagedupfinder scan ./photos --exact  # Find only byte-identical duplicates
  imagedupfinder scan ./photos --full   # Re-hash all files, ignore cache`,
	Args: cobra.ExactArgs(1),
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().BoolVar(&exactMode, "exact", false, "Use exact file hash matching instead of perceptual hashing")
	scanCmd.Flags().BoolVar(&fullRescan, "full", false, "Re-hash all files instead of skipping unchanged ones")
}

func runScan(cmd *cobra.Command, args []string) error {
	folder := args[0]

	// Resolve absolute path
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check folder exists
	info, err := os.Stat(absFolder)
	if err != nil {
		return fmt.Errorf("folder not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", absFolder)
	}

	fmt.Printf("Scanning: %s\n", absFolder)
	if exactMode {
		fmt.Println("Mode: Exact matching (SHA256)")
	} else {
		fmt.Printf("Mode: Perceptual hashing (threshold: %d)\n", threshold)
	}
	fmt.Printf("Workers: %d\n\n", workers)

	// Initialize storage
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	// Load previous results so unchanged files can skip re-hashing and stale
	// entries for deleted files can be pruned.
	knownImages, err := store.GetAllImages()
	if err != nil {
		return fmt.Errorf("failed to load previous scan results: %w", err)
	}
	knownByPath := make(map[string]*models.ImageInfo, len(knownImages))
	for _, img := range knownImages {
		knownByPath[img.Path] = img
	}

	// Create scanner with progress reporting
	lastLine := ""
	opts := []scan.Option{
		scan.WithWorkers(workers),
		scan.WithProgress(func(scanned, total int, current string) {
			// Clear previous line
			if lastLine != "" {
				fmt.Print("\r" + strings.Repeat(" ", len(lastLine)) + "\r")
			}
			shortPath := current
			if len(shortPath) > 50 {
				shortPath = "..." + shortPath[len(shortPath)-47:]
			}
			lastLine = fmt.Sprintf("Progress: %d/%d  %s", scanned, total, shortPath)
			fmt.Print(lastLine)
		}),
	}
	if !fullRescan {
		opts = append(opts, scan.WithKnownImages(knownByPath))
	}
	s := scan.NewScanner(opts...)

	// Scan folder
	images, err := s.ScanFolder(absFolder)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Clear progress line
	if lastLine != "" {
		fmt.Print("\r" + strings.Repeat(" ", len(lastLine)) + "\r")
	}

	// Reused entries are the exact pointers handed to the scanner via the
	// known-images map; anything else was freshly hashed.
	reused := 0
	scannedPaths := make(map[string]bool, len(images))
	for _, img := range images {
		scannedPaths[img.Path] = true
		if knownByPath[img.Path] == img {
			reused++
		}
	}

	fmt.Printf("Scanned: %d images", len(images))
	if reused > 0 {
		fmt.Printf(" (%d unchanged, skipped re-hashing)", reused)
	}
	fmt.Println()

	// Prune entries for files under this folder that no longer exist on disk,
	// so deleted files don't linger in list/serve output.
	pruned := 0
	prefix := absFolder + string(os.PathSeparator)
	for _, img := range knownImages {
		if scannedPaths[img.Path] || !strings.HasPrefix(img.Path, prefix) {
			continue
		}
		if _, err := os.Stat(img.Path); os.IsNotExist(err) {
			if store.DeleteImage(img.Path) == nil {
				pruned++
			}
		}
	}
	if pruned > 0 {
		fmt.Printf("Pruned: %d missing files removed from database\n", pruned)
	}

	if len(images) == 0 {
		fmt.Println("No images found.")
		return nil
	}

	// Compute file hashes if in exact mode (reused entries may already have one)
	if exactMode {
		fmt.Println("Computing file hashes...")
		for _, img := range images {
			if img.FileHash != "" {
				continue
			}
			fileHash, err := hash.ComputeFileHash(img.Path)
			if err == nil {
				img.FileHash = fileHash
			}
		}
	}

	// Save images to database
	if err := store.SaveImages(images); err != nil {
		return fmt.Errorf("failed to save images: %w", err)
	}

	// Find duplicate groups
	fmt.Println("Finding duplicates...")
	var matcher match.Matcher
	if exactMode {
		matcher = match.NewExactMatcher()
	} else {
		matcher = match.NewPerceptualMatcher(threshold)
	}
	groups := matcher.FindGroups(images)

	// Update groups in database
	if err := store.UpdateGroups(groups); err != nil {
		return fmt.Errorf("failed to update groups: %w", err)
	}

	// Record scan history
	totalDuplicates := 0
	for _, group := range groups {
		totalDuplicates += len(group.Remove)
	}
	store.RecordScan(absFolder, len(images), len(groups), totalDuplicates)

	// Print summary
	fmt.Println()
	fmt.Println("=== Scan Complete ===")
	fmt.Printf("Total images:     %d\n", len(images))
	fmt.Printf("Duplicate groups: %d\n", len(groups))
	fmt.Printf("Duplicates found: %d\n", totalDuplicates)

	if len(groups) > 0 {
		fmt.Println()
		fmt.Println("Run 'imagedupfinder list' to see duplicate groups")
		fmt.Println("Run 'imagedupfinder clean --dry-run' to preview deletions")
	}

	return nil
}
