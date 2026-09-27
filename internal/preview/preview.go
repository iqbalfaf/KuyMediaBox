// Package preview serves media previews to the UI: the file itself for the <video>/<audio>
// player (/kmb/media), one video frame as JPEG (/kmb/frame) and a waveform picture
// (/kmb/wave). The trim and crop dialogs use them.
package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"

	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/proc"
)

// Middleware handles the preview paths and passes everything else to next. ffmpeg returns
// the current ffmpeg.exe ("" when missing).
func Middleware(ffmpeg func() string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kmb/media":
			serveMedia(w, r)
		case "/kmb/frame":
			serveFrame(w, r, ffmpeg())
		case "/kmb/wave":
			serveWave(w, r, ffmpeg())
		case "/kmb/imgx":
			serveImage(w, r, ffmpeg())
		default:
			next.ServeHTTP(w, r)
		}
	})
}

var mediaTypes = map[string]string{
	".mp4": "video/mp4", ".m4v": "video/mp4", ".mov": "video/mp4", ".webm": "video/webm", ".ogv": "video/ogg", ".mkv": "video/x-matroska",
	".mp3": "audio/mpeg", ".m4a": "audio/mp4", ".aac": "audio/aac", ".wav": "audio/wav", ".flac": "audio/flac", ".ogg": "audio/ogg",
	".oga": "audio/ogg", ".opus": "audio/ogg",
}

// serveMedia streams a local file with range support so the player can seek.
func serveMedia(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	if t := mediaTypes[strings.ToLower(filepath.Ext(path))]; t != "" {
		w.Header().Set("Content-Type", t)
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

func num(q string, def float64) float64 {
	v, err := strconv.ParseFloat(q, 64)
	if err != nil || v < 0 {
		return def
	}
	return v
}

// serveFrame returns one frame at t seconds, w pixels wide, with the rotation (rot 0/90/
// 180/270) and mirroring (fh, fv) of the Video page applied.
func serveFrame(w http.ResponseWriter, r *http.Request, ff string) {
	if ff == "" {
		http.Error(w, "ffmpeg missing", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	width := int(num(q.Get("w"), 640))
	width = max(64, min(width, 1920))
	var chain []string
	switch q.Get("rot") {
	case "90":
		chain = append(chain, "transpose=1")
	case "180":
		chain = append(chain, "hflip", "vflip")
	case "270":
		chain = append(chain, "transpose=2")
	}
	if q.Get("fh") == "1" {
		chain = append(chain, "hflip")
	}
	if q.Get("fv") == "1" {
		chain = append(chain, "vflip")
	}
	chain = append(chain, fmt.Sprintf("scale=%d:-2", width))
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	t := strconv.FormatFloat(num(q.Get("t"), 0), 'f', 3, 64)
	out, err := bytesOut(ctx, ff, "-hide_banner", "-loglevel", "error", "-nostdin", "-ss", t, "-i", q.Get("path"),
		"-frames:v", "1", "-vf", strings.Join(chain, ","), "-f", "image2pipe", "-c:v", "mjpeg", "-q:v", "4", "-")
	if err != nil || len(out) == 0 {
		// Past the end (or a very short file): try the first frame.
		out, err = bytesOut(ctx, ff, "-hide_banner", "-loglevel", "error", "-nostdin", "-i", q.Get("path"),
			"-frames:v", "1", "-vf", strings.Join(chain, ","), "-f", "image2pipe", "-c:v", "mjpeg", "-q:v", "4", "-")
	}
	if err != nil || len(out) == 0 {
		http.Error(w, "no frame", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=600")
	_, _ = w.Write(out)
}

// serveWave draws the waveform of the first audio stream (w×h pixels, transparent PNG).
func serveWave(w http.ResponseWriter, r *http.Request, ff string) {
	if ff == "" {
		http.Error(w, "ffmpeg missing", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	width := max(200, min(int(num(q.Get("w"), 1200)), 4000))
	height := max(40, min(int(num(q.Get("h"), 120)), 600))
	color := q.Get("c")
	if len(color) != 6 || strings.Trim(strings.ToLower(color), "0123456789abcdef") != "" {
		color = "ff7a45"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	// The audio goes in through the filter's input label only: mapping it as well would add
	// an audio stream the image2pipe output can't encode.
	out, err := bytesOut(ctx, ff, "-hide_banner", "-loglevel", "error", "-nostdin", "-i", q.Get("path"),
		"-filter_complex", fmt.Sprintf("[0:a:0]aformat=channel_layouts=mono,showwavespic=s=%dx%d:colors=0x%s:scale=sqrt", width, height, color),
		"-frames:v", "1", "-f", "image2pipe", "-c:v", "png", "-")
	if err != nil || len(out) == 0 {
		http.Error(w, "no waveform", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=600")
	_, _ = w.Write(out)
}

// bytesOut runs a program and returns its raw standard output (pictures are binary).
func bytesOut(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := proc.Command(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// serveImage returns a picture (any format the Image page reads, upright by EXIF) w pixels
// wide with the page's rotation (rot) and mirroring (fh, fv) applied.
func serveImage(w http.ResponseWriter, r *http.Request, ff string) {
	q := r.URL.Query()
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	img, err := imageconv.Decode(ctx, q.Get("path"), ff)
	if err != nil {
		http.Error(w, "can't read the picture", http.StatusUnprocessableEntity)
		return
	}
	width := max(64, min(int(num(q.Get("w"), 1280)), 2400))
	if img.Bounds().Dx() > width {
		img = imaging.Resize(img, width, 0, imaging.Linear)
	}
	switch q.Get("rot") {
	case "90":
		img = imaging.Rotate270(img)
	case "180":
		img = imaging.Rotate180(img)
	case "270":
		img = imaging.Rotate90(img)
	}
	if q.Get("fh") == "1" {
		img = imaging.FlipH(img)
	}
	if q.Get("fv") == "1" {
		img = imaging.FlipV(img)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, imageconv.Flatten(img, image.White), &jpeg.Options{Quality: 85}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}
