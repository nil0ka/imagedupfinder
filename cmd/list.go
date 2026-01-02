package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"imagedupfinder/internal"
)

var (
	listJSON    bool
	listVerbose bool
	listSummary bool
	listLimit   int
	listOffset  int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all duplicate groups",
	Long: `Display all detected duplicate groups with their images.

Each group shows:
- Group ID
- Images in the group with their quality scores
- Which image will be kept (highest score) marked with ✓
- Which images will be removed marked with ✗

Example:
  imagedupfinder list              # Show first 10 groups (default)
  imagedupfinder list -n 0         # Show all groups
  imagedupfinder list -s           # Summary view (compact)
  imagedupfinder list --offset 10  # Groups 11-20`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
	listCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "Show detailed image info")
	listCmd.Flags().BoolVarP(&listSummary, "summary", "s", false, "Show summary only (group counts and sizes)")
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 10, "Limit number of groups to display (0 = all)")
	listCmd.Flags().IntVar(&listOffset, "offset", 0, "Skip first N groups (for pagination)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	store, err := internal.NewStorage(dbPath)
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
		fmt.Println("Run 'imagedupfinder scan <folder>' to scan for duplicates.")
		return nil
	}

	// Calculate totals
	totalDuplicates := 0
	var totalSavings int64
	for _, group := range groups {
		for _, img := range group.Remove {
			totalDuplicates++
			totalSavings += img.FileSize
		}
	}

	fmt.Printf("Found %d duplicate groups (%d duplicates, %s reclaimable)\n\n",
		len(groups), totalDuplicates, formatSize(totalSavings))

	// Apply pagination
	totalGroups := len(groups)
	startIdx := listOffset
	if startIdx > len(groups) {
		startIdx = len(groups)
	}
	groups = groups[startIdx:]

	if listLimit > 0 && listLimit < len(groups) {
		groups = groups[:listLimit]
	}

	// Display groups
	if len(groups) == 0 {
		fmt.Printf("No groups in range (offset %d exceeds total %d)\n", listOffset, totalGroups)
	} else if listSummary {
		printSummaryTable(groups)
	} else {
		for _, group := range groups {
			printGroup(group, listVerbose)
		}
	}

	// Show pagination info
	endIdx := startIdx + len(groups)
	if len(groups) > 0 {
		fmt.Printf("Showing groups %d-%d of %d\n", startIdx+1, endIdx, totalGroups)
		if endIdx < totalGroups {
			nextOffset := endIdx
			limitArg := ""
			if listLimit > 0 {
				limitArg = fmt.Sprintf(" -n %d", listLimit)
			}
			fmt.Printf("Next page: imagedupfinder list%s --offset %d\n", limitArg, nextOffset)
		}
	}

	fmt.Println()
	fmt.Println("Run 'imagedupfinder clean --dry-run' to preview deletions")
	fmt.Println("Run 'imagedupfinder clean' to remove duplicates")

	return nil
}

func printSummaryTable(groups []*internal.DuplicateGroup) {
	fmt.Printf("%-8s  %-8s  %-12s  %s\n", "Group", "Images", "Reclaimable", "Keep (best quality)")
	fmt.Println(strings.Repeat("-", 70))

	for _, group := range groups {
		var reclaimable int64
		for _, img := range group.Remove {
			reclaimable += img.FileSize
		}

		keepName := filepath.Base(group.Keep.Path)
		if len(keepName) > 35 {
			keepName = keepName[:32] + "..."
		}

		fmt.Printf("#%-7d  %-8d  %-12s  %s\n",
			group.ID, len(group.Images), formatSize(reclaimable), keepName)
	}
	fmt.Println()
}

func printGroup(group *internal.DuplicateGroup, verbose bool) {
	fmt.Printf("Group #%d (%d images)\n", group.ID, len(group.Images))
	fmt.Println(strings.Repeat("-", 60))

	for _, img := range group.Images {
		isKeep := img.Path == group.Keep.Path
		marker := "✗"
		if isKeep {
			marker = "✓"
		}

		shortPath := shortenPath(img.Path, 40)

		if verbose {
			fmt.Printf("  %s %s\n", marker, img.Path)
			fmt.Printf("      Resolution: %dx%d  Format: %s  Size: %s\n",
				img.Width, img.Height, strings.ToUpper(img.Format), formatSize(img.FileSize))
			fmt.Printf("      Score: %.0f\n", img.Score)
		} else {
			fmt.Printf("  %s %-40s  %dx%d  %-4s  %8s  Score: %.0f\n",
				marker, shortPath, img.Width, img.Height,
				strings.ToUpper(img.Format), formatSize(img.FileSize), img.Score)
		}
	}
	fmt.Println()
}

func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Try to show filename and as much of the path as possible
	dir, file := filepath.Split(path)
	if len(file) >= maxLen-3 {
		return "..." + file[len(file)-(maxLen-3):]
	}

	remaining := maxLen - len(file) - 4 // 4 for ".../"
	if remaining > 0 && len(dir) > remaining {
		dir = dir[len(dir)-remaining:]
	}
	return "..." + dir + file
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
