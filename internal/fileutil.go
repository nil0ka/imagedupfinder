package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MoveFile moves a file to the destination directory.
// If a file with the same name exists, it appends a counter (e.g., file_1.jpg).
func MoveFile(src, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	filename := filepath.Base(src)
	dest := filepath.Join(destDir, filename)

	// Handle name collision
	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(filename)
		name := strings.TrimSuffix(filename, ext)
		counter := 1
		for {
			dest = filepath.Join(destDir, fmt.Sprintf("%s_%d%s", name, counter, ext))
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				break
			}
			counter++
		}
	}

	return os.Rename(src, dest)
}
