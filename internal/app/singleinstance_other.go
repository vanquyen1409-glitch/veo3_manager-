//go:build !windows

package app

import "errors"

var ErrAlreadyRunning = errors.New("another instance is already running")

// AcquireSingleInstance is a no-op on non-Windows builds.
func AcquireSingleInstance(_ string) (release func(), err error) {
	return func() {}, nil
}
