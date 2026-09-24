//go:build windows

package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const (
	createNoWindow  = 0x08000000
	detachedProcess = 0x00000008
)

// psQuote quotes a string for a single-quoted PowerShell literal.
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// Apply installs a downloaded update. It starts a hidden helper that waits for this process
// to exit and then either restarts the swapped exe (portable) or runs the installer silently
// and restarts the installed app. The caller must quit the app right after Apply returns nil.
func Apply(downloaded, mode string) error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	pid := os.Getpid()
	var script string
	switch mode {
	case ModePortable:
		// A running exe cannot be overwritten, but it can be renamed.
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return fmt.Errorf("tidak bisa mengganti file aplikasi: %w", err)
		}
		if err := os.Rename(downloaded, exe); err != nil {
			_ = os.Rename(old, exe) // put the original back
			return fmt.Errorf("tidak bisa memasang versi baru: %w", err)
		}
		script = fmt.Sprintf("Wait-Process -Id %d -Timeout 60 -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 500; Start-Process -FilePath %s",
			pid, psQuote(exe))
	case ModeInstaller:
		script = fmt.Sprintf("Wait-Process -Id %d -Timeout 60 -ErrorAction SilentlyContinue; "+
			"$p = Start-Process -FilePath %s -ArgumentList '/S' -Verb RunAs -Wait -PassThru -ErrorAction SilentlyContinue; "+
			"Start-Sleep -Milliseconds 500; Start-Process -FilePath %s; Remove-Item -LiteralPath %s -ErrorAction SilentlyContinue",
			pid, psQuote(downloaded), psQuote(exe), psQuote(downloaded))
	default:
		return errors.New("mode update tidak dikenal")
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow | detachedProcess}
	if err := cmd.Start(); err != nil {
		if mode == ModePortable { // roll back so the app keeps working
			_ = os.Rename(exe, downloaded)
			_ = os.Rename(exe+".old", exe)
		}
		return fmt.Errorf("tidak bisa menjalankan pemasang update: %w", err)
	}
	return cmd.Process.Release()
}
