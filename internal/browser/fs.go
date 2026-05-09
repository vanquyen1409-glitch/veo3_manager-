package browser

import "os"

// removeAll is split into its own file so unit tests can stub it.
func removeAll(path string) error {
	return os.RemoveAll(path)
}
