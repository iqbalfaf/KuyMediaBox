package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/pdf"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/rename"
)

// Bulk rename page: names any files by a pattern, find & replace, added text, or after a
// field written inside PDF and Word documents (e.g. the student name on report cards).

// RenameItem is a file of the rename list.
type RenameItem struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Password string `json:"password"` // locked PDFs (content mode)
}

// RenamePlan is the proposed new name (without extension) of one file.
type RenamePlan struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Ext   string `json:"ext"` // extension with dot
	Error string `json:"error"`
}

// RenameRequest are the options of the page: how names are built, and the extension.
type RenameRequest struct {
	rename.Options
	ExtLower bool `json:"extLower"` // ".JPG" becomes ".jpg"
}

type docText struct {
	done  chan struct{} // closed once lines and err are set
	lines []string
	err   error
}

// The preview runs again on every option change, so the text of each document is kept
// (by path, size, time and password) until the app closes. A document being read is
// waited for, not read twice (old Word files go through Office, which is slow).
var docTexts = struct {
	sync.Mutex
	m map[string]*docText
}{m: map[string]*docText{}}

const maxDocTexts = 5000

// docLines reads (or recalls) a document's text; engines is only called for files that
// need Office or LibreOffice to be read.
func (a *App) docLines(ctx context.Context, in pdf.Input, engines func() pdf.Engines) ([]string, error) {
	st, err := os.Stat(in.Path)
	if err != nil {
		return nil, fmt.Errorf(i18n.L("File tidak ditemukan: %s", "File not found: %s"), in.Path)
	}
	k := fmt.Sprintf("%s|%d|%d|%s", strings.ToLower(in.Path), st.Size(), st.ModTime().UnixNano(), in.Password)
	docTexts.Lock()
	c, ok := docTexts.m[k]
	if !ok {
		if len(docTexts.m) >= maxDocTexts {
			docTexts.m = map[string]*docText{}
		}
		c = &docText{done: make(chan struct{})}
		docTexts.m[k] = c
	}
	docTexts.Unlock()
	if ok {
		select {
		case <-c.done:
			return c.lines, c.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var eng pdf.Engines
	if !pdf.ReadsNatively(in.Path) {
		eng = engines()
	}
	c.lines, c.err = pdf.DocLines(ctx, in, appdir.TempDir(), eng)
	close(c.done)
	if ctx.Err() != nil {
		// Cancelled: let the next call read it again.
		docTexts.Lock()
		if docTexts.m[k] == c {
			delete(docTexts.m, k)
		}
		docTexts.Unlock()
	}
	return c.lines, c.err
}

// RenamePreview proposes the new name of each file; {n} counts in the order given.
func (a *App) RenamePreview(items []RenameItem, o RenameRequest) []RenamePlan {
	engines := sync.OnceValue(func() pdf.Engines { return a.PdfEnvironment().Office })
	out := make([]RenamePlan, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, it := range items {
		wg.Add(1)
		go func(i int, it RenameItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			base := filepath.Base(it.Path)
			ext := filepath.Ext(base)
			f := rename.File{Name: strings.TrimSuffix(base, ext), Index: i}
			if st, err := os.Stat(it.Path); err == nil {
				f.Modified = st.ModTime()
			}
			if e := strings.ToLower(ext); e == ".pdf" || pdf.OfficeExts[e] == pdf.FamilyWord {
				f.Lines = func() ([]string, error) {
					return a.docLines(context.Background(), pdf.Input{Path: it.Path, Password: it.Password}, engines)
				}
			}
			if o.ExtLower {
				ext = strings.ToLower(ext)
			}
			out[i] = RenamePlan{ID: it.ID, Ext: ext}
			name, err := rename.NewName(f, o.Options)
			if err != nil {
				out[i].Error = failMessage(err)
				return
			}
			out[i].Name = name
		}(i, it)
	}
	wg.Wait()
	return out
}

// failMessage is the one-line message pdfFail would show.
func failMessage(err error) string {
	var ue *queue.UserError
	if errors.As(pdfFail(err), &ue) {
		return ue.Message
	}
	return err.Error()
}

// RenameApply renames the files in place, or copies them under their new names into
// copyDir when it is set.
func (a *App) RenameApply(jobs []rename.Job, copyDir string) ([]rename.Result, error) {
	if len(jobs) == 0 {
		return nil, errors.New(i18n.L("Tambahkan file dulu", "Add files first"))
	}
	return rename.Apply(jobs, copyDir), nil
}

// RenameUndo gives renamed files their old names back and deletes copies.
func (a *App) RenameUndo(done []rename.Result) []rename.Result {
	return rename.Undo(done)
}
