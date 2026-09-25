//go:build !windows

package platform

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// HideWindow is a no-op outside Windows; it puts the child in its own process group.
func HideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// KillTree kills the child's process group.
func KillTree(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

func opener() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

// OpenFolder shows a folder in the file manager.
func OpenFolder(dir string) error { return exec.Command(opener(), dir).Start() }

// RevealFile opens the file's folder.
func RevealFile(path string) error { return exec.Command(opener(), filepath.Dir(path)).Start() }

// ComRegistered is always false outside Windows.
func ComRegistered(string) bool { return false }

// ProcessIDs is not implemented outside Windows.
func ProcessIDs(string) map[uint32]bool { return map[uint32]bool{} }

// KillNew is a no-op outside Windows.
func KillNew(string, map[uint32]bool) {}

// SystemLightTheme is always false outside Windows.
func SystemLightTheme() bool { return false }

// Sleep is not supported outside Windows.
func Sleep() error { return errors.New("not supported") }

// Shutdown is not supported outside Windows.
func Shutdown() error { return errors.New("not supported") }

// SendToDir has no meaning outside Windows.
func SendToDir() string { return "" }

// CreateShortcut is not supported outside Windows.
func CreateShortcut(lnk, target, desc string) error { return errors.New("not supported") }
