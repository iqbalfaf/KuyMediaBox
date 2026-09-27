package mediaconv

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"kuymediabox/internal/i18n"
)

// SlideOptions turn pictures into an animation or a video (the Image page's "Animasi").
type SlideOptions struct {
	Format     string  `json:"format"`     // gif | webp | mp4
	Seconds    float64 `json:"seconds"`    // how long each picture shows
	Size       int     `json:"size"`       // longest side in pixels
	Ratio      string  `json:"ratio"`      // "" (first picture) | 1:1 | 16:9 | 9:16 | 4:5
	Fit        string  `json:"fit"`        // contain | cover | blur
	Background string  `json:"background"` // #rrggbb for contain
	Fade       float64 `json:"fade"`       // crossfade in seconds (0 = cut)
	Music      string  `json:"music"`      // MP4 only: background music
}

// MaxFadeSlides limits crossfades (one filter per picture).
const MaxFadeSlides = 200

// Normalize fixes invalid slideshow options.
func (o *SlideOptions) Normalize() {
	switch o.Format {
	case "webp", "mp4":
	default:
		o.Format = "gif"
	}
	if o.Seconds < 0.1 || o.Seconds > 60 {
		o.Seconds = 1
	}
	if o.Size < 64 || o.Size > 3840 {
		o.Size = 1080
	}
	switch o.Fit {
	case "cover", "blur":
	default:
		o.Fit = "contain"
	}
	if o.Fade < 0 || o.Fade >= o.Seconds {
		o.Fade = 0
	}
	if o.Format != "mp4" {
		o.Music = ""
	}
}

// SlideshowPlan encodes pictures already fitted to one canvas size (frames[i] shows for
// Seconds, crossfading into the next). work holds the frames; out is the result.
func SlideshowPlan(frames []string, out string, o SlideOptions) (Plan, error) {
	o.Normalize()
	n := len(frames)
	if n < 2 {
		return Plan{}, errors.New(i18n.L("Tambahkan minimal 2 gambar untuk animasi", "Add at least 2 pictures for an animation"))
	}
	fade := o.Fade
	if n > MaxFadeSlides {
		fade = 0
	}
	fps := "30"
	if o.Format == "gif" {
		fps = "15"
	}
	total := float64(n) * o.Seconds
	var args []string
	var graph string
	secs3 := func(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }
	if fade > 0 {
		// Every picture is a looped still of Seconds+fade; transition k starts at k·Seconds.
		for _, f := range frames {
			args = append(args, "-loop", "1", "-framerate", fps, "-t", secs3(o.Seconds+fade), "-i", f)
		}
		var b strings.Builder
		prev := "0:v"
		for k := 1; k < n; k++ {
			label := fmt.Sprintf("x%d", k)
			fmt.Fprintf(&b, "[%s][%d:v]xfade=transition=fade:duration=%s:offset=%s[%s];", prev, k, secs3(fade), secs3(float64(k)*o.Seconds), label)
			prev = label
		}
		graph = b.String() + "[" + prev + "]format=yuv420p,setsar=1"
		total += fade
	} else {
		// A picture sequence at 1/Seconds frames per second. GIF and WebP keep one frame per
		// picture (they store how long each shows); video needs a real frame rate.
		args = append(args, "-framerate", "1/"+secs3(o.Seconds), "-i", frameSequence(frames[0]))
		graph = "[0:v]format=yuv420p,setsar=1"
		if o.Format == "mp4" {
			graph = "[0:v]fps=" + fps + ",format=yuv420p,setsar=1"
		}
	}
	musicIn := -1
	if o.Music != "" {
		musicIn = 1 // after the picture sequence
		if fade > 0 {
			musicIn = n // after the n stills
		}
		args = append(args, "-stream_loop", "-1", "-i", o.Music)
	}
	switch o.Format {
	case "gif":
		graph = strings.Replace(graph, "format=yuv420p,setsar=1", "split[pa][pb];[pa]palettegen=stats_mode=diff[pp];[pb][pp]paletteuse=dither=bayer:bayer_scale=4", 1)
		args = append(args, "-filter_complex", graph+"[v]", "-map", "[v]", "-loop", "0", "-f", "gif")
	case "webp":
		graph = strings.Replace(graph, "format=yuv420p,setsar=1", "format=yuva420p", 1)
		args = append(args, "-filter_complex", graph+"[v]", "-map", "[v]", "-c:v", "libwebp_anim", "-lossless", "0", "-q:v", "80", "-loop", "0", "-f", "webp")
	default: // mp4
		args = append(args, "-filter_complex", graph+"[v]", "-map", "[v]", "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p", "-r", fps)
		if musicIn >= 0 {
			args = append(args, "-map", strconv.Itoa(musicIn)+":a:0", "-c:a", "aac", "-b:a", "192k",
				"-af", "afade=t=out:st="+secs3(max(0, total-2))+":d=2")
		}
		args = append(args, "-t", secs3(total), "-movflags", "+faststart")
	}
	return Plan{Passes: [][]string{append(args, out)}, Duration: total}, nil
}

// frameSequence turns "…\frame_0001.png" into the ffmpeg pattern "…\frame_%04d.png".
func frameSequence(first string) string {
	i := strings.LastIndex(first, "0001")
	if i < 0 {
		return first
	}
	return first[:i] + "%04d" + first[i+4:]
}
