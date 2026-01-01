package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"imagedupfinder/internal"
)

var scanCmd = &cobra.Command{
	Use:   "scan <folder>",
	Short: "Scan a folder for duplicate images",
	Long: `Scan a folder recursively for images and detect duplicates.

The scan will:
1. Find all supported images (jpg, png, gif, webp, etc.)
2. Compute perceptual hashes for each image
3. Group similar images based on hash distance
4. Store results in the database for later use

Example:
  imagedupfinder scan ./photos
  imagedupfinder scan /path/to/images --threshold 5`,
	Args: cobra.ExactArgs(1),
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
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
	fmt.Printf("Threshold: %d (Hamming distance)\n", threshold)
	fmt.Printf("Workers: %d\n\n", workers)

	// Initialize storage
	store, err := internal.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	// Create scanner with progress reporting
	lastLine := ""
	s := internal.NewScanner(
		internal.WithWorkers(workers),
		internal.WithProgress(func(scanned, total int, current string) {
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
	)

	// Scan folder
	images, err := s.ScanFolder(absFolder)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Clear progress line
	if lastLine != "" {
		fmt.Print("\r" + strings.Repeat(" ", len(lastLine)) + "\r")
	}

	fmt.Printf("Scanned: %d images\n", len(images))

	if len(images) == 0 {
		fmt.Println("No images found.")
		return nil
	}

	// Save images to database
	if err := store.SaveImages(images); err != nil {
		return fmt.Errorf("failed to save images: %w", err)
	}

	// Find duplicate groups
	fmt.Println("Finding duplicates...")
	g := internal.NewGrouper(threshold)
	groups := g.FindGroups(images)

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
