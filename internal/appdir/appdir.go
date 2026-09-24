// Package appdir resolves the folders KuyMediaBox uses for settings, tools and data.
package appdir

import (
	"os"
	"path/filepath"
)

const appName = "KuyMediaBox"

// ConfigDir is where settings live (%APPDATA%\KuyMediaBox).
func ConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return ensure(filepath.Join(base, appName))
}

// DataDir is where downloaded tools and caches live (%LOCALAPPDATA%\KuyMediaBox).
func DataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil || base == "" {
			base = os.TempDir()
		}
	}
	return ensure(filepath.Join(base, appName))
}

// ToolsDir holds tools the app downloaded itself.
func ToolsDir() string { return ensure(filepath.Join(DataDir(), "bin")) }

// TempDir is a scratch folder cleaned on startup.
func TempDir() string { return ensure(filepath.Join(DataDir(), "tmp")) }

// ExeDir is the folder that contains the running executable.
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// fallbackFolder is the usual folder name under the user profile, per module.
var fallbackFolder = map[string]string{"image": "Pictures", "video": "Videos", "audio": "Music", "download": "Downloads"}

// DefaultOutputDir is the default result folder of a module:
// Pictures\KuyMediaBox, Videos\KuyMediaBox, Music\KuyMediaBox or Downloads\KuyMediaBox.
func DefaultOutputDir(kind string) string {
	base := knownFolder(kind)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(DataDir(), "Hasil")
		}
		name := fallbackFolder[kind]
		if name == "" {
			name = "Documents"
		}
		base = filepath.Join(home, name)
	}
	return filepath.Join(base, appName)
}

func ensure(dir string) string {
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
