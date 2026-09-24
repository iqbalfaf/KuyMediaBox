//go:build windows

package appdir

import "golang.org/x/sys/windows"

// knownFolder returns a Windows known folder (follows OneDrive/library redirection).
func knownFolder(kind string) string {
	var id *windows.KNOWNFOLDERID
	switch kind {
	case "image":
		id = windows.FOLDERID_Pictures
	case "video":
		id = windows.FOLDERID_Videos
	case "audio":
		id = windows.FOLDERID_Music
	case "download":
		id = windows.FOLDERID_Downloads
	default:
		return ""
	}
	p, err := windows.KnownFolderPath(id, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return ""
	}
	return p
}
