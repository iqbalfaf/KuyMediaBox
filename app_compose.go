package main

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

// The Image page's "Animasi" and "Kolase" modes: many pictures become one file.

// StartSlideshow makes an animated GIF/WebP or an MP4 slideshow of all items, in order.
func (a *App) StartSlideshow(items []JobItem, o mediaconv.SlideOptions) ([]JobRef, error) {
	o.Normalize()
	if len(items) < 2 {
		return nil, errors.New(i18n.L("Tambahkan minimal 2 gambar untuk animasi", "Add at least 2 pictures for an animation"))
	}
	ff := a.tools.Path(tools.FFmpeg)
	if ff == "" {
		return nil, errNoFFmpeg()
	}
	if o.Music != "" {
		if _, err := os.Stat(o.Music); err != nil {
			return nil, fmt.Errorf(i18n.L("File musik tidak ditemukan: %s", "Music file not found: %s"), filepath.Base(o.Music))
		}
	}
	out, err := a.resolveOutput(queue.KindImage)
	if err != nil {
		return nil, err
	}
	paths, size := itemPaths(items)
	run := a.convertTaskSuffix(items[0], out, i18n.L("_animasi", "_animation"), o.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
		work, err := os.MkdirTemp(appdir.TempDir(), "slides-*")
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		defer os.RemoveAll(work)
		// Every picture is fitted to one canvas, sized from the first picture's ratio.
		r.Message(i18n.L("Menyiapkan gambar…", "Preparing the pictures…"))
		bg := color.Color(color.Black)
		if c, err := hexColor(o.Background); err == nil {
			bg = c
		}
		var frames []string
		var w, h int
		for i, p := range paths {
			img, err := imageconv.Decode(ctx, p, ff)
			if err != nil {
				return queue.Fail(fmt.Sprintf(i18n.L("Gambar tidak bisa dibaca: %s", "Picture can't be read: %s"), filepath.Base(p)), err.Error())
			}
			if i == 0 {
				w, h = imageconv.RatioSize(o.Ratio, img.Bounds(), o.Size)
			}
			frame := filepath.Join(work, fmt.Sprintf("frame_%04d.png", i+1))
			if err := savePNG(frame, imageconv.FitCanvas(img, w, h, o.Fit, bg)); err != nil {
				return queue.Fail(err.Error(), "")
			}
			frames = append(frames, frame)
			r.Progress(0.3 * float64(i+1) / float64(len(paths)))
		}
		plan, err := mediaconv.SlideshowPlan(frames, tmp, o)
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		plan.Dir = work
		return runPlan(ctx, ff, plan, progressFrom(r, 0.3, 0.7))
	})
	return a.addCombined(queue.KindImage, items, size, run), nil
}

// CollageJob is a collage plus how to save it.
type CollageJob struct {
	Layout  imageconv.CollageOptions `json:"layout"`
	Format  string                   `json:"format"` // jpg | png | webp
	Quality int                      `json:"quality"`
}

// StartCollage lays all items out in one picture.
func (a *App) StartCollage(items []JobItem, job CollageJob) ([]JobRef, error) {
	if len(items) < 2 {
		return nil, errors.New(i18n.L("Tambahkan minimal 2 gambar untuk kolase", "Add at least 2 pictures for a collage"))
	}
	switch job.Format {
	case "png", "webp":
	default:
		job.Format = "jpg"
	}
	out, err := a.resolveOutput(queue.KindImage)
	if err != nil {
		return nil, err
	}
	ff := a.tools.Path(tools.FFmpeg)
	paths, size := itemPaths(items)
	run := a.convertTaskSuffix(items[0], out, i18n.L("_kolase", "_collage"), job.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
		r.Message(i18n.L("Menyusun kolase…", "Building the collage…"))
		img, err := imageconv.Collage(ctx, paths, job.Layout, ff, func(p float64) { r.Progress(p * 0.9) })
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return queue.Fail(err.Error(), err.Error())
		}
		o := imageconv.Options{Format: job.Format, Quality: job.Quality, Background: job.Layout.Background}
		if err := imageconv.EncodeFile(ctx, img, tmp, o); err != nil {
			return queue.Fail(i18n.L("Kolase tidak bisa disimpan", "The collage can't be saved"), err.Error())
		}
		return nil
	})
	return a.addCombined(queue.KindImage, items, size, run), nil
}

func itemPaths(items []JobItem) ([]string, int64) {
	paths := make([]string, len(items))
	var size int64
	for i, it := range items {
		paths[i] = it.Path
		size += fileSize(it.Path)
	}
	return paths, size
}

// hexColor reads "#rrggbb".
func hexColor(s string) (color.Color, error) {
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return nil, err
	}
	return color.NRGBA{r, g, b, 255}, nil
}
