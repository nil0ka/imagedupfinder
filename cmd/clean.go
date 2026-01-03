package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"imagedupfinder/internal/fileutil"
	"imagedupfinder/internal/models"
	"imagedupfinder/internal/storage"
)

var (
	dryRun    bool
	moveTo    string
	permanent bool
	noConfirm bool
	groupIDs  []int
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove or move duplicate images",
	Long: `Remove duplicate images, keeping the highest quality version of each.

The clean command will:
1. Keep the image with the highest quality score in each group
2. Move lower quality duplicates to trash (default) or delete permanently

Options:
  --dry-run     Preview what would be removed without actually removing
  --permanent   Delete files permanently instead of moving to trash
  --move-to     Move duplicates to a specific folder
  --yes         Skip confirmation prompt
  --group       Specify group IDs to clean (can be used multiple times)

Example:
  imagedupfinder clean                     # Move to trash (default)
  imagedupfinder clean --permanent         # Delete permanently
  imagedupfinder clean --move-to=./backup  # Move to specific folder
  imagedupfinder clean --dry-run           # Preview only
  imagedupfinder clean --group=1 --group=3 # Clean only groups 1 and 3`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without removing")
	cleanCmd.Flags().BoolVar(&permanent, "permanent", false, "Delete permanently instead of moving to trash")
	cleanCmd.Flags().StringVar(&moveTo, "move-to", "", "Move duplicates to this folder")
	cleanCmd.Flags().BoolVarP(&noConfirm, "yes", "y", false, "Skip confirmation prompt")
	cleanCmd.Flags().IntSliceVarP(&groupIDs, "group", "g", nil, "Group IDs to clean (can be specified multiple times)")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	groups, err := store.GetDuplicateGroups()
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}

	if len(groups) == 0 {
		fmt.Println("No duplicate groups found.")
		return nil
	}

	// Filter groups if --group is specified
	if len(groupIDs) > 0 {
		groupIDSet := make(map[int]bool)
		for _, id := range groupIDs {
			groupIDSet[id] = true
		}

		var filtered []*models.DuplicateGroup
		for _, group := range groups {
			if groupIDSet[group.ID] {
				filtered = append(filtered, group)
			}
		}

		if len(filtered) == 0 {
			fmt.Printf("No matching groups found for IDs: %v\n", groupIDs)
			fmt.Println("Run 'imagedupfinder list' to see available group IDs.")
			return nil
		}

		groups = filtered
		fmt.Printf("Processing %d selected group(s): %v\n\n", len(groups), groupIDs)
	}

	// Collect files to remove
	var toRemove []string
	var totalSize int64
	for _, group := range groups {
		for _, img := range group.Remove {
			// Verify file still exists
			if _, err := os.Stat(img.Path); err == nil {
				toRemove = append(toRemove, img.Path)
				totalSize += img.FileSize
			}
		}
	}

	if len(toRemove) == 0 {
		fmt.Println("No files to remove (files may have been already deleted).")
		return nil
	}

	// Determine action
	var action string
	if moveTo != "" {
		action = fmt.Sprintf("move to %s", moveTo)
	} else if permanent {
		action = "permanently delete"
	} else {
		action = "move to trash"
	}

	fmt.Printf("Will %s %d files (%s)\n\n", action, len(toRemove), formatSize(totalSize))

	if dryRun {
		fmt.Println("Files to be removed:")
		for _, path := range toRemove {
			fmt.Printf("  %s\n", path)
		}
		fmt.Println()
		fmt.Println("(Dry run - no files were modified)")
		fmt.Println("Run without --dry-run to actually remove files.")
		return nil
	}

	// Confirm unless --yes flag is set
	if !noConfirm {
		fmt.Printf("Are you sure you want to %s %d files? [y/N]: ", action, len(toRemove))
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Create move-to directory if needed
	if moveTo != "" {
		if err := os.MkdirAll(moveTo, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", moveTo, err)
		}
	}

	// Process files
	var processed, failed int
	for _, path := range toRemove {
		var err error
		if moveTo != "" {
			err = fileutil.MoveFile(path, moveTo)
		} else if permanent {
			err = os.Remove(path)
		} else {
			err = fileutil.MoveToTrash(path)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to process %s: %v\n", path, err)
			failed++
		} else {
			processed++
			// Remove from database
			store.DeleteImage(path)
		}
	}

	fmt.Println()
	if moveTo != "" {
		fmt.Printf("Moved %d files to %s\n", processed, moveTo)
	} else if permanent {
		fmt.Printf("Permanently deleted %d files\n", processed)
	} else {
		fmt.Printf("Moved %d files to trash\n", processed)
	}
	if failed > 0 {
		fmt.Printf("Failed: %d files\n", failed)
	}
	fmt.Printf("Space reclaimed: %s\n", formatSize(totalSize))

	return nil
}
