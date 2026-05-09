//go:build windows

package download

import (
	"golang.org/x/sys/windows"
)

// DiskFreeBytes returns the free bytes available to the calling user at the
// given path. On Windows uses GetDiskFreeSpaceExW which respects user quotas.
// Returns (0, err) on failure - callers should treat 0 as "unknown" and not
// block downloads.
func DiskFreeBytes(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeForUser, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeForUser, &total, &totalFree); err != nil {
		return 0, err
	}
	return freeForUser, nil
}
