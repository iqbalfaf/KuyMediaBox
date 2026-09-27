package tools

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
)

// Tools added for subtitles, AI upscaling, background removal and PNG compression. They
// live in their own folders under the tools folder (except the single-file PNG tools).

// Tool IDs.
const (
	Whisper     = "whisper"
	RealESRGAN  = "realesrgan"
	OnnxRuntime = "onnxruntime"
	Oxipng      = "oxipng"
	Pngquant    = "pngquant"
)

// subdirs are the private folders of tools that come with DLLs or model files.
var subdirs = map[string]string{Whisper: "whisper", RealESRGAN: "realesrgan", OnnxRuntime: "onnxruntime"}

// ToolDir is the folder a tool is installed into.
func ToolDir(id string) string {
	if d := subdirs[id]; d != "" {
		return filepath.Join(appdir.ToolsDir(), d)
	}
	return appdir.ToolsDir()
}

// ModelDir is where the models of a feature are kept ("whisper" or "bgremove").
func ModelDir(kind string) string {
	switch kind {
	case "whisper":
		return filepath.Join(ToolDir(Whisper), "models")
	}
	return filepath.Join(appdir.ToolsDir(), "models", kind)
}

// versionFile records the release a folder tool was installed from.
func versionFile(id string) string { return filepath.Join(ToolDir(id), "VERSION") }

func readVersionFile(id string) string {
	b, err := os.ReadFile(versionFile(id))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}

// Download saves url to dest (resuming broken connections), reporting 0..1 progress.
func Download(ctx context.Context, url, dest string, progress func(float64)) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return downloadFile(ctx, url, dest, progress)
}

// ghRelease list of a repository (newest first).
func releases(ctx context.Context, repo string) ([]ghRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases?per_page=15", nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(i18n.L("GitHub menjawab %s", "GitHub replied %s"), resp.Status)
	}
	var list []ghRelease
	return list, json.NewDecoder(resp.Body).Decode(&list)
}

var reFeedTag = regexp.MustCompile(`/releases/tag/([A-Za-z0-9._-]+)`)

// feedTags reads the release tags (newest first) from a repository's Atom feed.
func feedTags(ctx context.Context, repo string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/"+repo+"/releases.atom", nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var tags []string
	seen := map[string]bool{}
	for _, m := range reFeedTag.FindAllStringSubmatch(string(body), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			tags = append(tags, m[1])
		}
	}
	return tags, nil
}

// whisperRelease finds the newest whisper.cpp build with a Windows zip. Builds are tagged
// "bNNNN"; the version shown is the newest "vX.Y.Z" tag.
func whisperRelease(ctx context.Context) (url, version string, err error) {
	list, err := releases(ctx, "ggml-org/whisper.cpp")
	if err != nil {
		// API limit (60 requests an hour without login): the release feed has no limit and
		// lists the build tags; their zip has a fixed name.
		tags, ferr := feedTags(ctx, "ggml-org/whisper.cpp")
		if ferr != nil {
			return "", "", err
		}
		for _, t := range tags {
			if url == "" && strings.HasPrefix(t, "b") {
				url = "https://github.com/ggml-org/whisper.cpp/releases/download/" + t + "/whisper-bin-x64.zip"
			}
			if version == "" && strings.HasPrefix(t, "v") {
				version = strings.TrimPrefix(t, "v")
			}
		}
		if url == "" {
			return "", "", err
		}
		if version == "" {
			version = "?"
		}
		return url, version, nil
	}
	for _, r := range list {
		if version == "" && strings.HasPrefix(r.TagName, "v") {
			version = strings.TrimPrefix(r.TagName, "v")
		}
		for _, a := range r.Assets {
			if url == "" && a.Name == "whisper-bin-x64.zip" {
				url = a.URL
			}
		}
	}
	if url == "" {
		return "", "", errors.New(i18n.L("file whisper untuk Windows tidak ditemukan di rilis terbaru", "whisper for Windows not found in the latest releases"))
	}
	if version == "" {
		version = "?"
	}
	return url, version, nil
}

const (
	realesrganURL     = "https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-windows.zip"
	realesrganVersion = "0.2.5.0"
	pngquantURL       = "https://pngquant.org/pngquant-windows.zip"
)

// installExtra installs the tools of this file; ok is false for other IDs.
func (m *Manager) installExtra(ctx context.Context, id string, progress func(float64)) (bool, error) {
	switch id {
	case Whisper:
		url, version, err := whisperRelease(ctx)
		if err != nil {
			return true, fmt.Errorf(i18n.L("tidak bisa membaca rilis whisper: %w", "can't read the whisper release: %w"), err)
		}
		// The program and its ggml DLLs (the test and example programs are left out).
		err = downloadZipInto(ctx, url, ToolDir(Whisper), func(name string) string {
			base := filepath.Base(name)
			low := strings.ToLower(base)
			if low == "whisper-cli.exe" || (strings.HasSuffix(low, ".dll") && low != "sdl2.dll") {
				return base
			}
			return ""
		}, progress)
		if err == nil {
			err = os.WriteFile(versionFile(Whisper), []byte(version), 0o644)
		}
		return true, err
	case RealESRGAN:
		err := downloadZipInto(ctx, realesrganURL, ToolDir(RealESRGAN), func(name string) string {
			low := strings.ToLower(name)
			switch {
			case strings.HasSuffix(low, ".exe") || strings.HasSuffix(low, "vcomp140.dll"):
				return filepath.Base(name)
			case strings.HasPrefix(low, "models/") && (strings.HasSuffix(low, ".bin") || strings.HasSuffix(low, ".param")):
				return filepath.Join("models", filepath.Base(name))
			}
			return ""
		}, progress)
		if err == nil {
			err = os.WriteFile(versionFile(RealESRGAN), []byte(realesrganVersion), 0o644)
		}
		return true, err
	case OnnxRuntime:
		rel, err := latestRelease(ctx, "microsoft/onnxruntime")
		if err != nil {
			return true, fmt.Errorf(i18n.L("tidak bisa membaca rilis ONNX Runtime: %w", "can't read the ONNX Runtime release: %w"), err)
		}
		v := strings.TrimPrefix(rel.TagName, "v")
		url := fmt.Sprintf("https://github.com/microsoft/onnxruntime/releases/download/%s/onnxruntime-win-x64-%s.zip", rel.TagName, v)
		err = downloadZipInto(ctx, url, ToolDir(OnnxRuntime), func(name string) string {
			low := strings.ToLower(filepath.Base(name))
			if low == "onnxruntime.dll" || low == "onnxruntime_providers_shared.dll" {
				return filepath.Base(name)
			}
			return ""
		}, progress)
		if err == nil {
			err = os.WriteFile(versionFile(OnnxRuntime), []byte(v), 0o644)
		}
		return true, err
	case Oxipng:
		rel, err := latestRelease(ctx, "oxipng/oxipng")
		if err != nil {
			return true, fmt.Errorf(i18n.L("tidak bisa membaca rilis oxipng: %w", "can't read the oxipng release: %w"), err)
		}
		v := strings.TrimPrefix(rel.TagName, "v")
		url := fmt.Sprintf("https://github.com/oxipng/oxipng/releases/download/%s/oxipng-%s-x86_64-pc-windows-msvc.zip", rel.TagName, v)
		return true, downloadZip(ctx, url, ToolDir(Oxipng), map[string]string{"oxipng.exe": "oxipng.exe"}, progress)
	case Pngquant:
		return true, downloadZip(ctx, pngquantURL, ToolDir(Pngquant), map[string]string{"pngquant.exe": "pngquant.exe"}, progress)
	}
	return false, nil
}

// checkExtraUpdates asks for the newest versions of the new tools.
func (m *Manager) checkExtraUpdates(ctx context.Context) {
	for id, repo := range map[string]string{Whisper: "ggml-org/whisper.cpp", OnnxRuntime: "microsoft/onnxruntime", Oxipng: "oxipng/oxipng"} {
		tag, err := latestTag(ctx, repo)
		if err != nil {
			continue
		}
		latest := strings.TrimPrefix(tag, "v")
		m.update(id, func(s *Status) {
			s.Latest = latest
			s.UpdateAvailable = s.Found && s.Source == "downloaded" && newer(latest, s.Version)
		})
	}
}

// downloadZipInto fetches a zip and extracts every entry pick maps to a path (relative to
// dir); "" skips the entry. The folder is replaced only when everything arrived.
func downloadZipInto(ctx context.Context, url, dir string, pick func(name string) string, progress func(float64)) error {
	tmp, err := os.CreateTemp(appdir.TempDir(), "tool-*.zip")
	if err != nil {
		return err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	if err := fetch(ctx, url, name, progress); err != nil {
		return err
	}
	zr, err := zip.OpenReader(name)
	if err != nil {
		return fmt.Errorf(i18n.L("arsip rusak: %w", "damaged archive: %w"), err)
	}
	defer zr.Close()
	stage := dir + ".new"
	_ = os.RemoveAll(stage)
	n := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rel := pick(strings.ReplaceAll(f.Name, `\`, "/"))
		if rel == "" || strings.Contains(rel, "..") {
			continue
		}
		dest := filepath.Join(stage, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := extractTo(f, dest); err != nil {
			return err
		}
		n++
	}
	if n == 0 {
		_ = os.RemoveAll(stage)
		return errors.New(i18n.L("isi arsip tidak dikenali", "unexpected archive contents"))
	}
	// Keep downloaded models (whisper keeps them inside its folder).
	if models := filepath.Join(dir, "models"); dirExists(models) && !dirExists(filepath.Join(stage, "models")) {
		_ = os.Rename(models, filepath.Join(stage, "models"))
	}
	old := dir + ".old"
	_ = os.RemoveAll(old)
	if dirExists(dir) {
		if err := os.Rename(dir, old); err != nil {
			_ = os.RemoveAll(stage)
			return fmt.Errorf(i18n.L("tidak bisa mengganti %s (sedang dipakai?): %w", "can't replace %s (in use?): %w"), filepath.Base(dir), err)
		}
	}
	if err := os.Rename(stage, dir); err != nil {
		_ = os.Rename(old, dir)
		return err
	}
	_ = os.RemoveAll(old)
	progress(1)
	return nil
}

func extractTo(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
