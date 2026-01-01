package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	dbPath    string
	threshold int
	workers   int
)

var rootCmd = &cobra.Command{
	Use:   "imagedupfinder",
	Short: "Find and manage duplicate images",
	Long: `imagedupfinder is a CLI tool for finding duplicate or similar images.

It uses perceptual hashing (pHash) to detect images that are similar even after
resizing or compression. The tool can automatically identify the best quality
image in each group based on resolution and format.

Example usage:
  imagedupfinder scan ./photos          # Scan a folder for duplicates
  imagedupfinder list                   # List all duplicate groups
  imagedupfinder clean --dry-run        # Preview what would be deleted
  imagedupfinder clean                  # Delete lower quality duplicates`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Default database path
	homeDir, _ := os.UserHomeDir()
	defaultDB := filepath.Join(homeDir, ".imagedupfinder", "images.db")

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDB, "Path to SQLite database")
	rootCmd.PersistentFlags().IntVar(&threshold, "threshold", 10, "Hamming distance threshold (0-64, lower = stricter)")
	rootCmd.PersistentFlags().IntVar(&workers, "workers", 8, "Number of parallel workers for scanning")
}
