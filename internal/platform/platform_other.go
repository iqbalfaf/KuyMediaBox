//go:build !windows

package platform

import (
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
