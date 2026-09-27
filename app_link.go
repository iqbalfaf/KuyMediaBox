package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/platform"
)

// Links from the browser: "kuymediabox://download?url=<link>" opens the Download page with
// that link (a bookmarklet or any page can call it once the scheme is registered).

const linkScheme = "kuymediabox"

// linksFromArgs picks the web links out of kuymediabox:// arguments.
func linksFromArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if !strings.HasPrefix(strings.ToLower(a), linkScheme+":") {
			continue
		}
		if l := linkFromURI(a); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// linkFromURI reads kuymediabox://download?url=… (or kuymediabox:https://… as a fallback).
func linkFromURI(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		if v := u.Query().Get("url"); v != "" {
			return cleanWebLink(v)
		}
	}
	rest := raw[len(linkScheme)+1:]
	rest = strings.TrimPrefix(rest, "//")
	if dec, err := url.QueryUnescape(rest); err == nil {
		rest = dec
	}
	return cleanWebLink(rest)
}

func cleanWebLink(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return ""
	}
	if len(s) > 4096 {
		return ""
	}
	return s
}

// GetLinkProtocol reports whether kuymediabox:// links open this copy of the app.
func (a *App) GetLinkProtocol() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	cmd := strings.ToLower(platform.URLProtocolCommand(linkScheme))
	return cmd != "" && strings.Contains(cmd, strings.ToLower(filepath.Clean(exe)))
}

// SetLinkProtocol registers or removes the kuymediabox:// link scheme for this user.
func (a *App) SetLinkProtocol(on bool) (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	if err := platform.SetURLProtocol(linkScheme, exe, "KuyMediaBox", on); err != nil {
		return a.GetLinkProtocol(), errors.New(i18n.L("Tidak bisa mengubah pendaftaran link: ", "Can't change the link registration: ") + err.Error())
	}
	return a.GetLinkProtocol(), nil
}

// TakeLaunchLinks returns the web links the app was started with (once).
func (a *App) TakeLaunchLinks() []string {
	a.launch.mu.Lock()
	defer a.launch.mu.Unlock()
	l := a.launch.links
	a.launch.links = nil
	if l == nil {
		l = []string{}
	}
	return l
}
