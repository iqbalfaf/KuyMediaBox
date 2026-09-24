package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

const libreStable = "https://download.documentfoundation.org/libreoffice/stable/"

var reLibreDir = regexp.MustCompile(`href="(\d+\.\d+\.\d+)/"`)

// latestLibreOffice reads the newest stable version from the download server listing.
func latestLibreOffice(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, libreStable, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LibreOffice: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	best := ""
	for _, m := range reLibreDir.FindAllStringSubmatch(string(body), -1) {
		if best == "" || newer(m[1], best) {
			best = m[1]
		}
	}
	if best == "" {
		return "", errors.New(i18n.L("versi LibreOffice tidak ditemukan", "no LibreOffice version found"))
	}
	return best, nil
}

// installLibreOffice downloads the official MSI and unpacks it with an administrative
// install (msiexec /a), which needs no admin rights and touches nothing outside our folder.
func installLibreOffice(ctx context.Context, progress func(float64)) error {
	v, err := latestLibreOffice(ctx)
	if err != nil {
		return fmt.Errorf(i18n.L("tidak bisa membaca versi LibreOffice: %w", "can't read the LibreOffice version: %w"), err)
	}
	url := fmt.Sprintf("%s%s/win/x86_64/LibreOffice_%s_Win_x86-64.msi", libreStable, v, v)
	msi := filepath.Join(appdir.TempDir(), "LibreOffice_"+v+".msi")
	defer os.Remove(msi)
	if err := fetch(ctx, url, msi, func(p float64) { progress(p * 0.85) }); err != nil {
		_ = os.Remove(msi)
		return err
	}
	dest := libreDir()
	staging := dest + ".new"
	_ = os.RemoveAll(staging)
	progress(0.88)
	ictx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	log := filepath.Join(appdir.TempDir(), "libreoffice-install.log")
	defer os.Remove(log)
	if _, err := proc.Output(ictx, "msiexec.exe", "/a", msi, "/qn", "/norestart", "TARGETDIR="+staging, "/L*", log); err != nil {
		_ = os.RemoveAll(staging)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf(i18n.L("gagal membuka paket LibreOffice: %w", "can't unpack LibreOffice: %w"), err)
	}
	found := false
	for _, p := range []string{filepath.Join(staging, "program", "soffice.com"), filepath.Join(staging, "LibreOffice", "program", "soffice.com")} {
		if fileExists(p) {
			found = true
		}
	}
	if !found {
		_ = os.RemoveAll(staging)
		return errors.New(i18n.L("soffice.com tidak ada di paket LibreOffice", "soffice.com is missing from the LibreOffice package"))
	}
	// The unpacked copy keeps its own copy of the MSI; it's not needed.
	if entries, err := os.ReadDir(staging); err == nil {
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".msi" {
				_ = os.Remove(filepath.Join(staging, e.Name()))
			}
		}
	}
	_ = os.RemoveAll(dest)
	if err := os.Rename(staging, dest); err != nil {
		return fmt.Errorf(i18n.L("tidak bisa memasang LibreOffice (sedang dipakai?): %w", "can't install LibreOffice (in use?): %w"), err)
	}
	progress(1)
	return nil
}
