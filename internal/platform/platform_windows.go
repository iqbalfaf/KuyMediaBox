//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// explorer starts Explorer with a raw command line. It must not get HideWindow: Explorer
// passes the hidden show state on to the folder window, which then opens invisibly.
func explorer(cmdLine string) error {
	c := exec.Command("explorer")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine}
	if err := c.Start(); err != nil {
		return err
	}
	// explorer.exe exits with code 1 even on success; just reap it.
	go func() { _ = c.Wait() }()
	return nil
}

// OpenFolder shows a folder in Explorer.
func OpenFolder(dir string) error {
	return explorer(`explorer "` + filepath.Clean(dir) + `"`)
}

// RevealFile opens Explorer with the file (or folder) selected.
func RevealFile(path string) error {
	// Explorer needs the raw "/select,<path>" argument without Go's quoting of the comma part.
	return explorer(`explorer /select,"` + filepath.Clean(path) + `"`)
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

// SystemLightTheme reports whether Windows apps use the light theme.
func SystemLightTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 1
}

// Sleep puts the computer to sleep.
func Sleep() error {
	proc := windows.NewLazySystemDLL("powrprof.dll").NewProc("SetSuspendState")
	if err := proc.Find(); err != nil {
		return err
	}
	r, _, err := proc.Call(0, 0, 0)
	if r == 0 {
		return err
	}
	return nil
}

// Shutdown turns the computer off (programs get the normal chance to save).
func Shutdown() error {
	c := exec.Command("shutdown", "/s", "/t", "0")
	HideWindow(c)
	return c.Run()
}

// SendToDir is the user's "Send to" menu folder.
func SendToDir() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\SendTo`)
}

// CreateShortcut writes a .lnk file pointing at target.
func CreateShortcut(lnk, target, desc string) error {
	script := `$s = (New-Object -ComObject WScript.Shell).CreateShortcut($env:KMB_LNK); ` +
		`$s.TargetPath = $env:KMB_TARGET; $s.IconLocation = $env:KMB_TARGET + ',0'; ` +
		`$s.Description = $env:KMB_DESC; $s.WorkingDirectory = [IO.Path]::GetDirectoryName($env:KMB_TARGET); $s.Save()`
	c := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	c.Env = append(os.Environ(), "KMB_LNK="+lnk, "KMB_TARGET="+target, "KMB_DESC="+desc)
	HideWindow(c)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetURLProtocol registers (or removes) a link scheme such as "kuymediabox://" for the
// current user, so browsers can hand links to exe.
func SetURLProtocol(scheme, exe, desc string, on bool) error {
	base := `Software\Classes\` + scheme
	if !on {
		for _, k := range []string{base + `\shell\open\command`, base + `\shell\open`, base + `\shell`, base + `\DefaultIcon`, base} {
			if err := registry.DeleteKey(registry.CURRENT_USER, k); err != nil && err != registry.ErrNotExist {
				return err
			}
		}
		return nil
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, base, registry.SET_VALUE)
	if err != nil {
		return err
	}
	_ = k.SetStringValue("", "URL:"+desc)
	err = k.SetStringValue("URL Protocol", "")
	k.Close()
	if err != nil {
		return err
	}
	if ik, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\DefaultIcon`, registry.SET_VALUE); err == nil {
		_ = ik.SetStringValue("", `"`+exe+`",0`)
		ik.Close()
	}
	ck, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer ck.Close()
	return ck.SetStringValue("", `"`+exe+`" "%1"`)
}

// URLProtocolCommand returns the command registered for a link scheme ("" when none).
func URLProtocolCommand(scheme string) string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\`+scheme+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("")
	return v
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// SetAutostart starts command (a quoted exe plus arguments) when the user signs in to
// Windows, or stops doing so.
func SetAutostart(name, command string, on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetStringValue(name, command)
}

// AutostartCommand returns the command started at sign-in ("" = none).
func AutostartCommand(name string) string {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return v
}

// AttachParentConsole lets a window-less run print to the console it was started from.
// Output that is already redirected (a pipe or file) keeps going there.
func AttachParentConsole() {
	const attachParent = ^uint32(0) // ATTACH_PARENT_PROCESS
	usable := func(h windows.Handle) bool {
		if h == 0 || h == windows.InvalidHandle {
			return false
		}
		t, err := windows.GetFileType(h)
		return err == nil && (t == windows.FILE_TYPE_DISK || t == windows.FILE_TYPE_PIPE || t == windows.FILE_TYPE_CHAR)
	}
	out, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	errH, _ := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
	if usable(out) && usable(errH) {
		return
	}
	attach := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := attach.Call(uintptr(attachParent)); r == 0 {
		return // started from Explorer: nowhere to print
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		if !usable(out) {
			os.Stdout = f
		}
		if !usable(errH) {
			os.Stderr = f
		}
	}
}
