//go:build windows

package internal

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	shFileOperationW = shell32.NewProc("SHFileOperationW")
)

const (
	foDelete           = 3
	fofAllowUndo       = 0x40
	fofNoConfirmation  = 0x10
	fofSilent          = 0x4
	fofNoErrorUI       = 0x400
)

// SHFILEOPSTRUCTW represents the Windows SHFILEOPSTRUCT structure.
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shfileopstructw
type shFileOpStructW struct {
	Hwnd                  uintptr
	Func                  uint32
	From                  *uint16
	To                    *uint16
	Flags                 uint16
	AnyOperationsAborted  int32
	NameMappings          uintptr
	ProgressTitle         *uint16
}

// moveToWindowsTrash moves a file to the Windows Recycle Bin.
func moveToWindowsTrash(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// SHFileOperationW requires double null-terminated string
	pathW, err := syscall.UTF16FromString(absPath + "\x00")
	if err != nil {
		return err
	}

	op := shFileOpStructW{
		Func:  foDelete,
		From:  &pathW[0],
		Flags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}

	ret, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperationW failed with code %d", ret)
	}

	return nil
}
