//go:build !windows

package download

import "golang.org/x/sys/unix"

// DiskFreeBytes returns the free bytes available at the given path. Uses the
// statfs syscall (works on Linux + macOS). Bavail is "free for unprivileged
// users" which matches what the user actually has to work with.
func DiskFreeBytes(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	// Bavail and Bsize are unsigned but signed-typed on darwin; cast both
	// to uint64 so the multiplication doesn't overflow on 32-bit systems.
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
