//go:build windows

package platform

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
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

// ComRegistered reports whether a COM automation class (e.g. "Word.Application") exists.
func ComRegistered(progID string) bool {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, progID+`\CLSID`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

// ProcessIDs returns the IDs of running processes with the given executable name.
func ProcessIDs(exe string) map[uint32]bool {
	out := map[uint32]bool{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return out
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), exe) {
			out[e.ProcessID] = true
		}
	}
	return out
}

// KillNew ends processes named exe that were not running before (orphans of a cancelled
// Office automation).
func KillNew(exe string, before map[uint32]bool) {
	for pid := range ProcessIDs(exe) {
		if !before[pid] {
			_ = KillTree(int(pid))
		}
	}
}
