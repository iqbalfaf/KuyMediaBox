package pdf

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"

	"kuymediabox/internal/i18n"
)

// Repair rebuilds a damaged PDF. The structure is rebuilt with pdfcpu first; when that
// fails, PDFium (which recovers broken cross-reference tables) re-saves the file.
func Repair(ctx context.Context, in Input, out, tmpDir string) error {
	if pc, err := readCtx(in.Path, in.Password); err == nil {
		if err := writeCtx(pc, out); err == nil {
			return nil
		}
	}
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		if errors.Is(err, ErrPassword) {
			return errLocked()
		}
		return errors.New(i18n.L("file terlalu rusak, tidak ada halaman yang bisa diselamatkan", "the file is too damaged; no pages could be recovered"))
	}
	defer doc.Close()
	if doc.Pages() == 0 {
		return errors.New(i18n.L("tidak ada halaman yang bisa diselamatkan", "no pages could be recovered"))
	}
	tmp := filepath.Join(tmpDir, "repair-"+randomHex(6)+".pdf")
	defer os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	err = doc.SaveCopy(w)
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	// Normalise the recovered file once more; fall back to PDFium's copy as is.
	if pc, err := readCtx(tmp, ""); err == nil {
		if err := writeCtx(pc, out); err == nil {
			return nil
		}
	}
	return os.Rename(tmp, out)
}
