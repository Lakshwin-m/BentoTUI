// Package winutil holds small Windows-specific helpers shared by panes.
package winutil

import (
	"os"
	"syscall"
)

// HasHiddenAttr reports whether a directory entry has the Windows hidden
// attribute. On non-Windows builds it is always false.
func HasHiddenAttr(e os.DirEntry) bool {
	info, err := e.Info()
	if err != nil {
		return false
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	return data.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}
