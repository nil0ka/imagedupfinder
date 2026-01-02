package internal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// MoveFile moves a file to the destination directory.
// If a file with the same name exists, it appends a counter (e.g., file_1.jpg).
func MoveFile(src, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	filename := filepath.Base(src)
	destName := findUniqueName(filename, func(name string) bool {
		_, err := os.Stat(filepath.Join(destDir, name))
		return os.IsNotExist(err)
	})

	return moveFileAcrossFS(src, filepath.Join(destDir, destName))
}

// findUniqueName finds a unique filename by appending a counter if needed.
// isAvailable should return true if the name can be used.
func findUniqueName(filename string, isAvailable func(string) bool) string {
	if isAvailable(filename) {
		return filename
	}

	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	for counter := 1; ; counter++ {
		candidate := fmt.Sprintf("%s_%d%s", name, counter, ext)
		if isAvailable(candidate) {
			return candidate
		}
	}
}

// moveFileAcrossFS moves a file, falling back to copy+delete for cross-filesystem moves.
func moveFileAcrossFS(src, dest string) error {
	err := os.Rename(src, dest)
	if err == nil {
		return nil
	}

	// Check if it's a cross-device link error
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		if errors.Is(linkErr.Err, syscall.EXDEV) {
			// Cross-filesystem: copy then delete
			if err := copyFile(src, dest); err != nil {
				return err
			}
			return os.Remove(src)
		}
	}

	return err
}

// copyFile copies a file from src to dest.
func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		os.Remove(dest) // Clean up on failure
		return err
	}

	return nil
}

// MoveToTrash moves a file to the system trash/recycle bin.
// - macOS: ~/.Trash
// - Linux: ~/.local/share/Trash (freedesktop.org spec)
// - Windows: Recycle Bin (via shell32.dll)
func MoveToTrash(src string) error {
	switch runtime.GOOS {
	case "windows":
		return moveToWindowsTrash(src)
	case "linux":
		trashDir, err := getTrashDir()
		if err != nil {
			return err
		}
		return moveToLinuxTrash(src, trashDir)
	default: // darwin, etc.
		trashDir, err := getTrashDir()
		if err != nil {
			return err
		}
		return MoveFile(src, trashDir)
	}
}

// getTrashDir returns the path to the system trash directory.
func getTrashDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	var trashDir string
	switch runtime.GOOS {
	case "darwin":
		trashDir = filepath.Join(homeDir, ".Trash")
	case "linux":
		trashDir = filepath.Join(homeDir, ".local", "share", "Trash", "files")
	default:
		// Windows and others: use a fallback folder
		trashDir = filepath.Join(homeDir, "imagedupfinder_trash")
	}

	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create trash directory: %w", err)
	}

	return trashDir, nil
}

// moveToLinuxTrash moves a file to Linux trash with proper .trashinfo metadata.
func moveToLinuxTrash(src, trashFilesDir string) error {
	homeDir, _ := os.UserHomeDir()
	trashInfoDir := filepath.Join(homeDir, ".local", "share", "Trash", "info")

	if err := os.MkdirAll(trashInfoDir, 0755); err != nil {
		return err
	}

	filename := filepath.Base(src)
	absPath, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	// Find unique name (must check both files dir and info dir)
	destName := findUniqueName(filename, func(name string) bool {
		_, err1 := os.Stat(filepath.Join(trashFilesDir, name))
		_, err2 := os.Stat(filepath.Join(trashInfoDir, name+".trashinfo"))
		return os.IsNotExist(err1) && os.IsNotExist(err2)
	})

	dest := filepath.Join(trashFilesDir, destName)
	infoPath := filepath.Join(trashInfoDir, destName+".trashinfo")

	// Create .trashinfo file
	info := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		absPath,
		time.Now().Format("2006-01-02T15:04:05"))

	if err := os.WriteFile(infoPath, []byte(info), 0644); err != nil {
		return err
	}

	// Move the file
	if err := moveFileAcrossFS(src, dest); err != nil {
		os.Remove(infoPath) // Clean up .trashinfo if move fails
		return err
	}

	return nil
}
