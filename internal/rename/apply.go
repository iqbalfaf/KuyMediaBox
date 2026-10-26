package rename

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/naming"
)

// Job is one file to rename.
type Job struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"` // new name without extension
	Ext  string `json:"ext"`  // new extension with dot; "" keeps the current one
}

// Result says what happened to one file.
type Result struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"` // the new path ("" when nothing was written)
	Copy    bool   `json:"copy"`
	Skipped bool   `json:"skipped"`
	Error   string `json:"error"`
}

func key(p string) string { return strings.ToLower(filepath.Clean(p)) }

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// free returns dir/name+ext, or "name (1)", "name (2)", … when that is taken.
func free(dir, name, ext string, taken map[string]bool) string {
	p := filepath.Join(dir, name+ext)
	for i := 1; (exists(p) || taken[key(p)]) && i < 100000; i++ {
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
	}
	taken[key(p)] = true
	return p
}

// Apply renames the files in their own folder, or copies them under the new name into
// copyDir when it is set. Existing files are never overwritten: a taken name gets a
// number. Renames go through a temporary name first, so files may swap names (A→B, B→A).
func Apply(jobs []Job, copyDir string) []Result {
	out := make([]Result, len(jobs))
	taken := map[string]bool{}
	type moved struct {
		i              int
		tmp, dir, name string
		ext            string
	}
	var todo []moved
	if copyDir != "" {
		if err := os.MkdirAll(copyDir, 0o755); err != nil {
			for i, j := range jobs {
				out[i] = Result{ID: j.ID, From: j.Path, Copy: true, Error: err.Error()}
			}
			return out
		}
	}
	for i, j := range jobs {
		r := &out[i]
		*r = Result{ID: j.ID, From: j.Path, Copy: copyDir != ""}
		name := naming.SanitizeFileName(strings.TrimSpace(j.Name))
		ext := j.Ext
		if ext == "" {
			ext = filepath.Ext(j.Path)
		}
		if strings.TrimSpace(j.Name) == "" {
			r.Error = i18n.L("Nama baru kosong", "The new name is empty")
			continue
		}
		if !exists(j.Path) {
			r.Error = i18n.L("File tidak ditemukan", "File not found")
			continue
		}
		if copyDir != "" {
			r.To = free(copyDir, name, ext, taken)
			if err := copyFile(j.Path, r.To); err != nil {
				r.To, r.Error = "", err.Error()
			}
			continue
		}
		dir := filepath.Dir(j.Path)
		if filepath.Join(dir, name+ext) == filepath.Clean(j.Path) {
			r.Skipped, r.Error = true, i18n.L("Nama sudah sesuai", "Already has this name")
			taken[key(j.Path)] = true
			continue
		}
		tmp := filepath.Join(dir, fmt.Sprintf(".kmb-rename-%d-%d%s", os.Getpid(), i, ext))
		if err := os.Rename(j.Path, tmp); err != nil {
			r.Error = renameErr(err)
			continue
		}
		todo = append(todo, moved{i, tmp, dir, name, ext})
	}
	for _, m := range todo {
		r := &out[m.i]
		target := free(m.dir, m.name, m.ext, taken)
		if err := os.Rename(m.tmp, target); err != nil {
			_ = os.Rename(m.tmp, r.From)
			r.Error = renameErr(err)
			continue
		}
		r.To = target
	}
	return out
}

// Undo reverses the results of Apply: renamed files get their old name back, copies are
// deleted.
func Undo(done []Result) []Result {
	var jobs []Job
	var out []Result
	for _, r := range done {
		if r.To == "" || r.Error != "" || r.Skipped {
			continue
		}
		if r.Copy {
			res := Result{ID: r.ID, From: r.To, Copy: true}
			if err := os.Remove(r.To); err != nil && !os.IsNotExist(err) {
				res.Error = err.Error()
			}
			out = append(out, res)
			continue
		}
		base := filepath.Base(r.From)
		ext := filepath.Ext(base)
		jobs = append(jobs, Job{ID: r.ID, Path: r.To, Name: strings.TrimSuffix(base, ext), Ext: ext})
	}
	return append(out, Apply(jobs, "")...)
}

func renameErr(err error) string {
	if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "used by another process") {
		return i18n.L("File sedang dibuka program lain atau tidak boleh diubah", "The file is open in another program or can't be changed")
	}
	return err.Error()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := naming.TempPath(dst)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, in); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return naming.Commit(tmp, dst)
}
