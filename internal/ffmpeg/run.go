package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/queue"
)

// Progress receives 0..1 progress and ffmpeg's speed string (e.g. "2.1x").
type Progress func(p float64, speed string)

// Run executes ffmpeg with args (input/output included) and reports progress based on duration.
func Run(ctx context.Context, ffmpegPath string, args []string, duration float64, onProgress Progress) error {
	return RunIn(ctx, ffmpegPath, "", args, duration, onProgress)
}

// RunIn is Run with a working directory (for relative pass logs and subtitle files).
func RunIn(ctx context.Context, ffmpegPath, dir string, args []string, duration float64, onProgress Progress) error {
	full := append([]string{"-hide_banner", "-nostdin", "-y", "-loglevel", "error", "-progress", "pipe:1", "-nostats"}, args...)
	cmd := proc.Command(ctx, ffmpegPath, full...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(i18n.L("ffmpeg tidak bisa dijalankan: %w", "ffmpeg can't be started: %w"), err)
	}

	tail := proc.NewTail(40)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		proc.ScanLines(stderr, tail.Add)
	}()
	go func() {
		defer wg.Done()
		var speed string
		proc.ScanLines(stdout, func(line string) {
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return
			}
			switch k {
			case "out_time_us", "out_time_ms": // both are microseconds in ffmpeg
				us, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
				if err != nil || us < 0 || onProgress == nil {
					return
				}
				if duration > 0 {
					onProgress(float64(us)/1e6/duration, speed)
				} else {
					onProgress(-1, speed)
				}
			case "speed":
				speed = strings.TrimSpace(v)
			case "progress":
				if strings.TrimSpace(v) == "end" && onProgress != nil {
					onProgress(1, speed)
				}
			}
		})
	}()
	wg.Wait()
	err = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return friendly(tail.String(), err)
	}
	return nil
}

var reNoise = regexp.MustCompile(`^\[[^\]]+ @ [0-9a-fx]+\]\s*`)

// friendly turns ffmpeg stderr into a short Indonesian message plus the raw detail.
func friendly(stderr string, err error) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = err.Error()
	}
	low := strings.ToLower(detail)
	msg := ""
	switch {
	case strings.Contains(low, "could not find tag for codec") ||
		strings.Contains(low, "not currently supported in container") ||
		strings.Contains(low, "codec not currently supported") ||
		strings.Contains(low, "are supported for webm") ||
		strings.Contains(low, "could not write header") ||
		strings.Contains(low, "incompatible with output"):
		msg = i18n.L("Codec tidak cocok dengan format ini. Pilih encode ulang atau format lain.", "The codec doesn't fit this format. Choose re-encoding or another format.")
	case strings.Contains(low, "invalid data found when processing input") || strings.Contains(low, "moov atom not found"):
		msg = i18n.L("File rusak atau tidak bisa dibaca", "File is damaged or can't be read")
	case strings.Contains(low, "unknown encoder") || strings.Contains(low, "encoder not found"):
		msg = i18n.L("Encoder tidak tersedia di FFmpeg ini", "Encoder not available in this FFmpeg")
	case strings.Contains(low, "no space left"):
		msg = i18n.L("Ruang disk penuh", "Disk is full")
	case strings.Contains(low, "permission denied") || strings.Contains(low, "access is denied"):
		msg = i18n.L("Tidak punya izin menulis ke folder hasil", "No permission to write to the output folder")
	case strings.Contains(low, "does not contain any stream") || strings.Contains(low, "output file does not contain"):
		msg = i18n.L("Tidak ada stream yang bisa dikonversi di file ini", "No convertible stream in this file")
	case strings.Contains(low, "matches no streams"):
		msg = i18n.L("File ini tidak punya audio/video yang dibutuhkan", "This file lacks the required audio/video")
	}
	if msg == "" {
		lines := strings.Split(detail, "\n")
		last := strings.TrimSpace(reNoise.ReplaceAllString(lines[len(lines)-1], ""))
		if last == "" || strings.HasPrefix(strings.ToLower(last), "conversion failed") && len(lines) > 1 {
			last = strings.TrimSpace(reNoise.ReplaceAllString(lines[max(0, len(lines)-2)], ""))
		}
		msg = i18n.L("Konversi gagal: ", "Conversion failed: ") + last
		if len([]rune(msg)) > 140 {
			msg = string([]rune(msg)[:140]) + "…"
		}
	}
	return queue.Fail(msg, detail)
}

// Encoders lists the encoder names ffmpeg supports.
func Encoders(ctx context.Context, ffmpegPath string) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := proc.Output(ctx, ffmpegPath, "-hide_banner", "-encoders")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	started := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "------") {
			started = true
			continue
		}
		if !started {
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 2 {
			set[f[1]] = true
		}
	}
	if len(set) == 0 {
		return nil, errors.New(i18n.L("daftar encoder kosong", "empty encoder list"))
	}
	return set, nil
}
