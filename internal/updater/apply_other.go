//go:build !windows

package updater

import "errors"

// Apply is only implemented for Windows.
func Apply(downloaded, mode string) error { return errors.New("update otomatis hanya untuk Windows") }
