//go:build !windows

package appdir

func knownFolder(kind string) string { return "" }
