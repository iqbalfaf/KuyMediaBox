package ffmpeg

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/proc"
)

var (
	reSilStart = regexp.MustCompile(`silence_start:\s*(-?[\d.]+)`)
	reSilEnd   = regexp.MustCompile(`silence_end:\s*(-?[\d.]+)`)
)

// Silence runs ffmpeg's silencedetect and returns where the sound starts and ends (seconds).
// end is 0 when the file does not end in silence.
func Silence(ctx context.Context, ffmpegPath, path string, noiseDB float64, duration float64) (start, end float64, err error) {
	args := []string{"-hide_banner", "-nostdin", "-i", path, "-vn", "-sn", "-dn",
		"-af", "silencedetect=noise=" + strconv.FormatFloat(noiseDB, 'f', 0, 64) + "dB:d=0.2", "-f", "null", "-"}
	cmd := proc.Command(ctx, ffmpegPath, args...)
	var out strings.Builder
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return 0, 0, ctx.Err()
		}
		return 0, 0, friendly(out.String(), err)
	}
	return parseSilence(out.String(), duration)
}

func parseSilence(log string, duration float64) (start, end float64, err error) {
	type span struct{ a, b float64 }
	var spans []span
	for _, line := range strings.Split(log, "\n") {
		if m := reSilStart.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			spans = append(spans, span{a: v, b: -1})
		}
		if m := reSilEnd.FindStringSubmatch(line); m != nil && len(spans) > 0 {
			v, _ := strconv.ParseFloat(m[1], 64)
			spans[len(spans)-1].b = v
		}
	}
	if len(spans) == 0 {
		return 0, 0, nil
	}
	if first := spans[0]; first.a <= 0.05 && first.b > 0 {
		start = first.b
	}
	last := spans[len(spans)-1]
	if last.b < 0 || (duration > 0 && last.b >= duration-0.05) {
		if last.a > start {
			end = last.a
		}
	}
	return start, end, nil
}

// Hardware encoders KuyMediaBox can use, per vendor.
var hwCandidates = map[string][]string{
	"nvenc": {"h264_nvenc", "hevc_nvenc", "av1_nvenc"},
	"qsv":   {"h264_qsv", "hevc_qsv", "av1_qsv", "vp9_qsv"},
	"amf":   {"h264_amf", "hevc_amf", "av1_amf"},
}

// HWEncoders tries every hardware encoder ffmpeg lists and returns the ones that really work
// on this PC (a listed encoder can still fail without the GPU or driver).
func HWEncoders(ctx context.Context, ffmpegPath string, listed map[string]bool) map[string]bool {
	out := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, names := range hwCandidates {
		for _, name := range names {
			if !listed[name] {
				continue
			}
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				c, cancel := context.WithTimeout(ctx, 20*time.Second)
				defer cancel()
				pix := "yuv420p"
				if strings.HasSuffix(name, "_qsv") || strings.HasSuffix(name, "_amf") {
					pix = "nv12"
				}
				_, err := proc.Output(c, ffmpegPath, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=black:s=640x360:r=30:d=0.3",
					"-frames:v", "5", "-pix_fmt", pix, "-c:v", name, "-f", "null", "-")
				if err == nil {
					mu.Lock()
					out[name] = true
					mu.Unlock()
				}
			}(name)
		}
	}
	wg.Wait()
	return out
}
