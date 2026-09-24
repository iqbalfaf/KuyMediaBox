// Package ffmpeg wraps ffprobe/ffmpeg: media info, progress-aware runs and encoder detection.
package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"kuymediabox/internal/proc"
)

// Info describes a media file.
type Info struct {
	Format        string  `json:"format"`
	Duration      float64 `json:"duration"` // seconds
	Bitrate       int64   `json:"bitrate"`
	HasVideo      bool    `json:"hasVideo"` // a real video stream, not cover art
	HasAudio      bool    `json:"hasAudio"`
	Width         int     `json:"width"` // as displayed (rotation applied)
	Height        int     `json:"height"`
	VideoCodec    string  `json:"videoCodec"`
	FPS           float64 `json:"fps"`
	AudioCodec    string  `json:"audioCodec"`
	SampleRate    int     `json:"sampleRate"`
	Channels      int     `json:"channels"`
	BitsPerSample int     `json:"bitsPerSample"`
	CoverIndex    int     `json:"coverIndex"` // stream index of attached picture, -1 if none
	CoverCodec    string  `json:"coverCodec"`
}

type probeStream struct {
	Index            int               `json:"index"`
	CodecType        string            `json:"codec_type"`
	CodecName        string            `json:"codec_name"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	AvgFrameRate     string            `json:"avg_frame_rate"`
	RFrameRate       string            `json:"r_frame_rate"`
	SampleRate       string            `json:"sample_rate"`
	Channels         int               `json:"channels"`
	BitsPerRawSample string            `json:"bits_per_raw_sample"`
	BitsPerSample    int               `json:"bits_per_sample"`
	SampleFmt        string            `json:"sample_fmt"`
	Duration         string            `json:"duration"`
	Disposition      map[string]int    `json:"disposition"`
	Tags             map[string]string `json:"tags"`
	SideDataList     []struct {
		Rotation float64 `json:"rotation"`
	} `json:"side_data_list"`
}

type probeOut struct {
	Streams []probeStream `json:"streams"`
	Format  struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
}

// Probe runs ffprobe on path.
func Probe(ctx context.Context, ffprobe, path string) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := proc.Output(ctx, ffprobe, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", "--", path)
	if err != nil {
		return Info{CoverIndex: -1}, fmt.Errorf("file tidak bisa dibaca: %w", err)
	}
	return parseProbe([]byte(out))
}

func parseProbe(data []byte) (Info, error) {
	var p probeOut
	info := Info{CoverIndex: -1}
	if err := json.Unmarshal(data, &p); err != nil {
		return info, fmt.Errorf("output ffprobe tidak valid: %w", err)
	}
	info.Format = p.Format.FormatName
	info.Duration = parseFloat(p.Format.Duration)
	info.Bitrate, _ = strconv.ParseInt(p.Format.BitRate, 10, 64)
	for _, s := range p.Streams {
		switch s.CodecType {
		case "video":
			if s.Disposition["attached_pic"] == 1 {
				if info.CoverIndex < 0 {
					info.CoverIndex = s.Index
					info.CoverCodec = s.CodecName
				}
				continue
			}
			if info.HasVideo {
				continue
			}
			info.HasVideo = true
			info.VideoCodec = s.CodecName
			info.Width, info.Height = s.Width, s.Height
			if rot := rotation(s); rot == 90 || rot == 270 {
				info.Width, info.Height = info.Height, info.Width
			}
			info.FPS = parseRate(s.AvgFrameRate)
			if info.FPS <= 0 || info.FPS > 1000 {
				info.FPS = parseRate(s.RFrameRate)
			}
			if info.Duration <= 0 {
				info.Duration = parseFloat(s.Duration)
			}
		case "audio":
			if info.HasAudio {
				continue
			}
			info.HasAudio = true
			info.AudioCodec = s.CodecName
			info.SampleRate, _ = strconv.Atoi(s.SampleRate)
			info.Channels = s.Channels
			info.BitsPerSample, _ = strconv.Atoi(s.BitsPerRawSample)
			if info.BitsPerSample == 0 {
				info.BitsPerSample = s.BitsPerSample
			}
			if info.BitsPerSample == 0 {
				switch strings.TrimSuffix(s.SampleFmt, "p") {
				case "s32", "flt":
					info.BitsPerSample = 24
				}
			}
			if info.Duration <= 0 {
				info.Duration = parseFloat(s.Duration)
			}
		}
	}
	if !info.HasVideo && !info.HasAudio {
		return info, fmt.Errorf("tidak ada video atau audio di file ini")
	}
	return info, nil
}

func rotation(s probeStream) int {
	r := 0.0
	for _, sd := range s.SideDataList {
		if sd.Rotation != 0 {
			r = sd.Rotation
		}
	}
	if r == 0 {
		if v, err := strconv.ParseFloat(s.Tags["rotate"], 64); err == nil {
			r = v
		}
	}
	deg := int(math.Round(math.Abs(r))) % 360
	return deg
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

func parseRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return parseFloat(s)
	}
	n, d := parseFloat(num), parseFloat(den)
	if d == 0 {
		return 0
	}
	return n / d
}
