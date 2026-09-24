//go:build !windows

package updater

import (
	"errors"

	"kuymediabox/internal/i18n"
)

// Apply is only implemented for Windows.
func Apply(downloaded, mode string) error {
	return errors.New(i18n.L("update otomatis hanya untuk Windows", "automatic updates are Windows-only"))
}
