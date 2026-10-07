package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/queue"
)

// startSubExtract queues saving the subtitle tracks inside videos as their own files.
func (a *App) startSubExtract(items []JobItem, o mediaconv.SubExtractOptions, out naming.OutputSpec, ff, probe string) ([]JobRef, error) {
	o.Normalize()
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		specs[i] = queue.Spec{Title: filepath.Base(it.Path), Input: it.Path, InSize: fileSize(it.Path), Run: a.subExtractTask(it, o, itemOut(out, it.OutDir), ff, probe)}
	}
	return refs(items, a.queue.AddMany(queue.KindVideo, specs)), nil
}

func (a *App) subExtractTask(it JobItem, o mediaconv.SubExtractOptions, out naming.OutputSpec, ff, probe string) queue.RunFunc {
	return func(ctx context.Context, r queue.Reporter) error {
		if _, err := os.Stat(it.Path); err != nil {
			return queue.Fail(i18n.L("File asli tidak ditemukan (dipindah atau dihapus?)", "Source file not found (moved or deleted?)"), err.Error())
		}
		info, err := ffmpeg.Probe(ctx, probe, it.Path)
		if err != nil {
			return queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
		}
		if len(info.Subs) == 0 {
			return queue.Skip(i18n.L("Tidak ada subtitle di dalam file ini", "This file has no subtitles inside"))
		}
		chosen := mediaconv.MatchLangs(info.Subs, o.Langs)
		if len(chosen) == 0 {
			return queue.Skip(fmt.Sprintf(i18n.L("Tidak ada subtitle berbahasa %s", "No subtitles in %s"), o.Langs))
		}
		subs := mediaconv.SubTargets(chosen, o.Format)
		if len(subs) == 0 {
			return queue.Fail(fmt.Sprintf(i18n.L("Jenis subtitle ini (%s) tidak bisa disimpan sebagai file", "This kind of subtitle (%s) can't be saved as a file"), chosen[0].Codec), "")
		}

		work, err := os.MkdirTemp(appdir.TempDir(), "subx-*")
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		defer os.RemoveAll(work)
		hasASS := false
		for i := range subs {
			subs[i].Path = filepath.Join(work, fmt.Sprintf("track%d.%s", i, subs[i].Ext))
			hasASS = hasASS || subs[i].Ext == "ass"
		}
		// Attached fonts only matter to ASS subtitles.
		var fonts []ffmpeg.Attachment
		var fontNames, fontOuts []string
		if o.Fonts && hasASS && len(info.Fonts) > 0 {
			fonts = info.Fonts
			fontNames = mediaconv.FontNames(fonts)
			if err := os.Mkdir(filepath.Join(work, "fonts"), 0o755); err != nil {
				return queue.Fail(err.Error(), "")
			}
			for _, n := range fontNames {
				fontOuts = append(fontOuts, filepath.Join(work, "fonts", n))
			}
		}

		r.Message(i18n.L("Mengekstrak subtitle…", "Extracting subtitles…"))
		if err := ffmpeg.RunIn(ctx, ff, work, mediaconv.SubExtractArgs(it.Path, subs, fonts, fontOuts), info.Duration, func(p float64, _ string) {
			if p >= 0 {
				r.Progress(p * 0.95)
			}
		}); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		// Subtitle files next to (or in the folder of) the video, named for players. Every
		// name stays reserved until the task ends, so two tracks never get the same file.
		s := a.cfg.Get()
		suffixes := mediaconv.SubSuffixes(subs)
		var releases []func()
		defer func() {
			for _, release := range releases {
				release()
			}
		}()
		var first string
		saved := 0
		for i, t := range subs {
			if _, err := os.Stat(t.Path); err != nil {
				return queue.Fail(i18n.L("Subtitle gagal diekstrak", "The subtitles couldn't be extracted"), err.Error())
			}
			if t.Encoder == "srt" { // made from ASS: drop the font tags (their sizes are in ASS units)
				if data, err := os.ReadFile(t.Path); err == nil {
					_ = os.WriteFile(t.Path, mediaconv.CleanSRT(data), 0o644)
				}
			}
			target, release, err := a.namer.Reserve(it.Path, out, suffixes[i], t.Ext, s.Conflict)
			if errors.Is(err, naming.ErrExists) {
				continue
			}
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			releases = append(releases, release)
			if err := moveFile(t.Path, target); err != nil {
				return queue.Fail(i18n.L("Tidak bisa menyimpan file hasil", "Can't save the output file"), err.Error())
			}
			a.markProduced(target)
			saved++
			if first == "" {
				first = target
			}
		}
		if saved == 0 {
			return queue.Skip(i18n.L("File hasil sudah ada", "Output file already exists"))
		}

		msg := fmt.Sprintf(i18n.L("%d subtitle", "%d subtitles"), saved)
		if len(fontOuts) > 0 {
			n, err := a.saveFonts(it.Path, out, s.Conflict, fontOuts, fontNames)
			if err != nil {
				return err
			}
			switch {
			case n > 0:
				msg += fmt.Sprintf(i18n.L(" · %d font", " · %d fonts"), n)
			case n < 0:
				msg += i18n.L(" · folder font sudah ada", " · fonts folder already exists")
			}
		}
		if skipped := len(chosen) - len(subs); skipped > 0 {
			msg += fmt.Sprintf(i18n.L(" · %d dilewati", " · %d skipped"), skipped)
		}
		r.SetOutput(first, fileSize(first))
		r.Message(msg)
		return nil
	}
}

// saveFonts moves the dumped fonts into "<name>_fonts" next to the subtitles, following the
// conflict setting for an existing folder. It returns how many were saved, or -1 when the
// folder already exists and conflict is skip.
func (a *App) saveFonts(video string, out naming.OutputSpec, conflict string, files, names []string) (int, error) {
	var have []int
	for i, f := range files {
		if _, err := os.Stat(f); err == nil {
			have = append(have, i) // an attachment ffmpeg couldn't write is left out
		}
	}
	if len(have) == 0 {
		return 0, nil
	}
	dir, err := naming.OutputDir(video, out)
	if err != nil {
		return 0, queue.Fail(err.Error(), "")
	}
	base := naming.SanitizeFileName(strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))) + "_fonts"
	folder := filepath.Join(dir, base)
	// Mkdir (not MkdirAll) claims the name, so two tasks never share a folder.
	err = os.Mkdir(folder, 0o755)
	if errors.Is(err, os.ErrExist) {
		switch conflict {
		case config.ConflictSkip:
			return -1, nil
		case config.ConflictOverwrite:
			err = nil
		default:
			for k := 2; errors.Is(err, os.ErrExist); k++ {
				folder = filepath.Join(dir, fmt.Sprintf("%s (%d)", base, k))
				err = os.Mkdir(folder, 0o755)
			}
		}
	}
	if err != nil {
		return 0, queue.Fail(i18n.L("Tidak bisa membuat folder font", "Can't create the fonts folder"), err.Error())
	}
	for _, i := range have {
		if err := moveFile(files[i], filepath.Join(folder, names[i])); err != nil {
			return 0, queue.Fail(i18n.L("Tidak bisa menyimpan font", "Can't save the fonts"), err.Error())
		}
	}
	return len(have), nil
}

// moveFile moves src to dst, copying when they are on different drives.
func moveFile(src, dst string) error {
	tmp := naming.TempPath(dst)
	if err := os.Rename(src, tmp); err != nil {
		return copyFile(src, dst)
	}
	return naming.Commit(tmp, dst)
}
