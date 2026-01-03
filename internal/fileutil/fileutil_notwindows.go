//go:build !windows

package fileutil

import "errors"

// moveToWindowsTrash is a stub for non-Windows platforms.
// This function should never be called on non-Windows systems.
func moveToWindowsTrash(path string) error {
	return errors.New("Windows Recycle Bin is not available on this platform")
}
