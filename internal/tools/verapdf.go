package tools

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

// veraPDF (PDF/A validator) is a Java program. The app installs it with a private Java
// runtime (Eclipse Temurin JRE) into its tools folder, without admin rights.

const (
	veraMaven = "https://repo1.maven.org/maven2/org/verapdf/apps/installer/"
	jreURL    = "https://api.adoptium.net/v3/binary/latest/21/ga/windows/x64/jre/hotspot/normal/eclipse"
)

func veraDir() string { return filepath.Join(appdir.ToolsDir(), "verapdf") }
func jreDir() string  { return filepath.Join(appdir.ToolsDir(), "jre") }

func veraCandidates() []struct{ path, source string } {
	var out []struct{ path, source string }
	add := func(p, source string) { out = append(out, struct{ path, source string }{p, source}) }
	add(filepath.Join(veraDir(), "verapdf.bat"), "downloaded")
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "USERPROFILE"} {
		if base := os.Getenv(env); base != "" {
			add(filepath.Join(base, "veraPDF", "verapdf.bat"), "system")
			add(filepath.Join(base, "verapdf", "verapdf.bat"), "system")
		}
	}
	return out
}

// JavaPath finds java.exe: the app's own runtime, JAVA_HOME, then PATH.
func JavaPath() string {
	own := filepath.Join(jreDir(), "bin", "java.exe")
	if fileExists(own) {
		return own
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		if p := filepath.Join(home, "bin", "java.exe"); fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath("java"); err == nil {
		return p
	}
	return ""
}

var reVeraJar = regexp.MustCompile(`(?i)^cli-(\d+(?:\.\d+)+)\.jar$`)

// veraVersion reads the version from the installed jar name (starting Java would be slow).
func veraVersion(bat string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(filepath.Dir(bat), "bin"))
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if m := reVeraJar.FindStringSubmatch(e.Name()); m != nil {
			if JavaPath() == "" {
				return "", errors.New(i18n.L("Java tidak ditemukan — klik Unduh untuk memasang Java bawaan", "Java not found — click Download to install a private Java"))
			}
			return m[1], nil
		}
	}
	return "", errors.New(i18n.L("instalasi veraPDF tidak lengkap", "incomplete veraPDF installation"))
}

func latestVeraPDF(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, veraMaven+"maven-metadata.xml", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var meta struct {
		Versioning struct {
			Release string `xml:"release"`
			Latest  string `xml:"latest"`
		} `xml:"versioning"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", err
	}
	v := meta.Versioning.Release
	if v == "" {
		v = meta.Versioning.Latest
	}
	if v == "" {
		return "", errors.New("no veraPDF release")
	}
	return v, nil
}

// installVeraPDF downloads a Java runtime when none is present, then veraPDF, and runs the
// veraPDF installer unattended into the tools folder.
func installVeraPDF(ctx context.Context, progress func(float64)) error {
	java := JavaPath()
	base := 0.0
	if java == "" {
		if err := installJRE(ctx, func(p float64) { progress(p * 0.55) }); err != nil {
			return fmt.Errorf(i18n.L("Java tidak bisa dipasang: %w", "Java couldn't be installed: %w"), err)
		}
		java = filepath.Join(jreDir(), "bin", "java.exe")
		base = 0.55
	}
	version, err := latestVeraPDF(ctx)
	if err != nil {
		return fmt.Errorf(i18n.L("tidak bisa membaca rilis veraPDF: %w", "can't read the veraPDF release: %w"), err)
	}
	tmp, err := os.MkdirTemp(appdir.TempDir(), "verapdf-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	zipPath := filepath.Join(tmp, "installer.zip")
	url := fmt.Sprintf("%s%s/installer-%s-installer.zip", veraMaven, version, version)
	if err := fetch(ctx, url, zipPath, func(p float64) { progress(base + p*(0.95-base)) }); err != nil {
		return err
	}
	jar, err := extractMatching(zipPath, tmp, regexp.MustCompile(`(?i)izpack-installer.*\.jar$`))
	if err != nil {
		return err
	}
	target := veraDir()
	_ = os.RemoveAll(target)
	props := filepath.Join(tmp, "install.properties")
	// Java properties treat "\" as an escape: forward slashes are safe on Windows.
	if err := os.WriteFile(props, []byte("INSTALL_PATH="+filepath.ToSlash(target)+"\n"), 0o644); err != nil {
		return err
	}
	ictx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	out, err := proc.Output(ictx, java, "-jar", jar, "-options", props)
	if err != nil {
		return fmt.Errorf(i18n.L("pemasang veraPDF gagal: %w", "the veraPDF installer failed: %w"), err)
	}
	if !fileExists(filepath.Join(target, "verapdf.bat")) {
		return fmt.Errorf(i18n.L("veraPDF tidak terpasang: %s", "veraPDF was not installed: %s"), firstLine(out))
	}
	progress(1)
	return nil
}

// installJRE extracts Eclipse Temurin's Java runtime into the tools folder.
func installJRE(ctx context.Context, progress func(float64)) error {
	tmp, err := os.CreateTemp(appdir.TempDir(), "jre-*.zip")
	if err != nil {
		return err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	if err := fetch(ctx, jreURL, name, func(p float64) { progress(p * 0.9) }); err != nil {
		return err
	}
	staging := jreDir() + ".new"
	_ = os.RemoveAll(staging)
	if err := extractAll(name, staging, true); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if !fileExists(filepath.Join(staging, "bin", "java.exe")) {
		_ = os.RemoveAll(staging)
		return errors.New(i18n.L("java.exe tidak ada di arsip", "java.exe is missing from the archive"))
	}
	_ = os.RemoveAll(jreDir())
	if err := os.Rename(staging, jreDir()); err != nil {
		return err
	}
	progress(1)
	return nil
}

// extractAll unpacks a zip into dest; strip drops the archive's top folder.
func extractAll(zipPath, dest string, strip bool) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf(i18n.L("arsip rusak: %w", "damaged archive: %w"), err)
	}
	defer zr.Close()
	root := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if strip {
			if i := strings.IndexAny(name, `\/`); i >= 0 {
				name = name[i+1:]
			} else {
				continue
			}
		}
		if name == "" {
			continue
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, root) {
			return fmt.Errorf("unsafe path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// extractMatching unpacks the first file whose name matches re into dir and returns its path.
func extractMatching(zipPath, dir string, re *regexp.Regexp) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf(i18n.L("arsip rusak: %w", "damaged archive: %w"), err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !re.MatchString(f.Name) {
			continue
		}
		target := filepath.Join(dir, filepath.Base(f.Name))
		return target, writeZipFile(f, target)
	}
	return "", errors.New(i18n.L("file pemasang tidak ada di arsip", "the installer file is missing from the archive"))
}
