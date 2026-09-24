//go:build windows

package platform

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

// HideWindow stops a child process from flashing a console window.
func HideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}

// KillTree terminates a process and every child it started (yt-dlp → ffmpeg, etc).
func KillTree(pid int) error {
	c := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	HideWindow(c)
	return c.Run()
}

// OpenFolder shows a folder in Explorer.
func OpenFolder(dir string) error {
	c := exec.Command("explorer", dir)
	HideWindow(c)
	// explorer.exe returns exit code 1 even on success.
	_ = c.Run()
	return nil
}

// RevealFile opens Explorer with the file selected.
func RevealFile(path string) error {
	c := exec.Command("explorer")
	HideWindow(c)
	// explorer needs the raw "/select,<path>" argument without Go's quoting of the comma part.
	c.SysProcAttr.CmdLine = `explorer /select,"` + path + `"`
	_ = c.Run()
	return nil
}
