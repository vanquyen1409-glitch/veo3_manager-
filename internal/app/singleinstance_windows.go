//go:build windows

package app

import (
	"errors"

	"golang.org/x/sys/windows"
)

// ErrAlreadyRunning is returned when another instance already holds the mutex.
var ErrAlreadyRunning = errors.New("another VEO3 Manager instance is already running")

// AcquireSingleInstance creates a named global mutex; if it exists, another
// instance owns it and the caller should exit.
func AcquireSingleInstance(name string) (release func(), err error) {
	n, err := windows.UTF16PtrFromString(`Global\` + name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, n)
	if err != nil {
		return nil, err
	}
	// CreateMutex returns a valid handle even when the named mutex already
	// exists; the duplicate case is signalled via GetLastError.
	if le := windows.GetLastError(); le == windows.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(h)
		return nil, ErrAlreadyRunning
	}
	return func() { _ = windows.CloseHandle(h) }, nil
}
