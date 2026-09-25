//go:build windows

package check

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

func diskUsage(path string) (free, total uint64, err error) {
	root := path
	if vol := filepath.VolumeName(path); vol != "" {
		root = vol + `\`
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel.NewProc("GetDiskFreeSpaceExW")
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, 0, err
	}
	var freeAvail, tot, totFree uint64
	r, _, e := proc.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&freeAvail)),
		uintptr(unsafe.Pointer(&tot)),
		uintptr(unsafe.Pointer(&totFree)),
	)
	if r == 0 {
		if e != syscall.Errno(0) {
			return 0, 0, e
		}
		return 0, 0, syscall.EINVAL
	}
	return freeAvail, tot, nil
}
