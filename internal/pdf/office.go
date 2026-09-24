package pdf

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/platform"
	"kuymediabox/internal/proc"
)

//go:embed scripts/office.ps1
var officeScript []byte

// Office document families.
const (
	FamilyWord  = "word"
	FamilyExcel = "excel"
	FamilyPPT   = "powerpoint"
)

// OfficeExts maps accepted extensions to their family.
var OfficeExts = map[string]string{
	".doc": FamilyWord, ".docx": FamilyWord, ".docm": FamilyWord, ".dot": FamilyWord, ".dotx": FamilyWord,
	".odt": FamilyWord, ".rtf": FamilyWord, ".wpd": FamilyWord,
	".xls": FamilyExcel, ".xlsx": FamilyExcel, ".xlsm": FamilyExcel, ".xlsb": FamilyExcel, ".ods": FamilyExcel, ".csv": FamilyExcel,
	".ppt": FamilyPPT, ".pptx": FamilyPPT, ".pptm": FamilyPPT, ".pps": FamilyPPT, ".ppsx": FamilyPPT, ".odp": FamilyPPT,
}

// ErrNoOffice means neither Microsoft Office nor LibreOffice is available.
var ErrNoOffice = errors.New("no office")

var comClass = map[string]string{FamilyWord: "Word.Application", FamilyExcel: "Excel.Application", FamilyPPT: "PowerPoint.Application"}
var comExe = map[string]string{FamilyWord: "WINWORD.EXE", FamilyExcel: "EXCEL.EXE", FamilyPPT: "POWERPNT.EXE"}

// Engines says which office programs can convert documents.
type Engines struct {
	Word        bool   `json:"word"`
	Excel       bool   `json:"excel"`
	PowerPoint  bool   `json:"powerpoint"`
	LibreOffice string `json:"libreoffice"` // path of soffice.com, "" when missing
}

// DetectEngines checks Microsoft Office (COM) and the given LibreOffice path.
func DetectEngines(soffice string) Engines {
	return Engines{
		Word:        platform.ComRegistered(comClass[FamilyWord]),
		Excel:       platform.ComRegistered(comClass[FamilyExcel]),
		PowerPoint:  platform.ComRegistered(comClass[FamilyPPT]),
		LibreOffice: soffice,
	}
}

func (e Engines) has(family string) bool {
	switch family {
	case FamilyWord:
		return e.Word
	case FamilyExcel:
		return e.Excel
	case FamilyPPT:
		return e.PowerPoint
	}
	return false
}

// One document at a time per program: Office automation and the LibreOffice profile don't
// like parallel use.
var (
	officeMu sync.Mutex
	libreMu  sync.Mutex
)

func runMSOffice(ctx context.Context, family, mode, in, out, tmpDir string, limit time.Duration) error {
	officeMu.Lock()
	defer officeMu.Unlock()
	script, err := writeScript(tmpDir, "kmb-office.ps1", officeScript)
	if err != nil {
		return err
	}
	before := platform.ProcessIDs(comExe[family])
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	_, err = proc.Output(ctx, "powershell.exe", powershell(ctx, script, "-App", family, "-Mode", mode, "-In", in, "-Out", out)...)
	if err != nil {
		// A killed script leaves its Office process behind; end it.
		platform.KillNew(comExe[family], before)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "password") {
			return errors.New(i18n.L("dokumen dilindungi password — buka proteksinya dulu di Office", "the document is password protected — remove the protection in Office first"))
		}
		return fmt.Errorf("Microsoft Office: %s", proc.LastLines(msg, 3))
	}
	if _, err := os.Stat(out); err != nil {
		return errors.New(i18n.L("Office tidak menghasilkan file", "Office produced no file"))
	}
	return nil
}

// runLibreOffice converts in with LibreOffice (headless) and moves the result to out.
func runLibreOffice(ctx context.Context, soffice, in, out, tmpDir, filter string, extra ...string) error {
	libreMu.Lock()
	defer libreMu.Unlock()
	outDir, err := os.MkdirTemp(tmpDir, "lo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(outDir)
	profile := "file:///" + filepath.ToSlash(filepath.Join(appdir.DataDir(), "libreoffice-profile"))
	args := []string{"--headless", "--invisible", "--norestore", "--nolockcheck", "--nodefault", "--nologo",
		"-env:UserInstallation=" + profile}
	args = append(args, extra...)
	args = append(args, "--convert-to", filter, "--outdir", outDir, in)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if _, err := proc.Output(ctx, soffice, args...); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("LibreOffice: %s", proc.LastLines(err.Error(), 3))
	}
	ext := strings.SplitN(filter, ":", 2)[0]
	got := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))+"."+ext)
	if _, err := os.Stat(got); err != nil {
		return errors.New(i18n.L("LibreOffice tidak menghasilkan file (format tidak didukung atau file rusak)", "LibreOffice produced no file (unsupported format or damaged file)"))
	}
	_ = os.Remove(out)
	return os.Rename(got, out)
}

// OfficeToPDF converts a Word, Excel or PowerPoint file. It returns the engine it used.
func OfficeToPDF(ctx context.Context, in, out, tmpDir string, eng Engines) (string, error) {
	family := OfficeExts[strings.ToLower(filepath.Ext(in))]
	if family == "" {
		return "", errors.New(i18n.L("format dokumen tidak didukung", "unsupported document format"))
	}
	var msErr error
	if eng.has(family) {
		if msErr = runMSOffice(ctx, family, "pdf", in, out, tmpDir, 15*time.Minute); msErr == nil {
			return "Microsoft Office", nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	if eng.LibreOffice != "" {
		if err := runLibreOffice(ctx, eng.LibreOffice, in, out, tmpDir, "pdf"); err != nil {
			return "", err
		}
		return "LibreOffice", nil
	}
	if msErr != nil {
		return "", msErr
	}
	return "", ErrNoOffice
}

// PDFToWord converts a PDF into an editable Word document: with Microsoft Word when present
// (best layout), else LibreOffice, else the built-in text converter.
func PDFToWord(ctx context.Context, in Input, out, tmpDir string, eng Engines, prog Progress) (string, error) {
	src := in.Path
	if in.Password != "" || isEncrypted(ctx, in.Path) {
		pc, err := readCtx(in.Path, in.Password)
		if err != nil {
			return "", err
		}
		src = filepath.Join(tmpDir, "plain-"+randomHex(6)+".pdf")
		if err := writeCtx(pc, src); err != nil {
			return "", err
		}
		defer os.Remove(src)
	}
	if eng.Word {
		// Word's PDF import can stall on a hidden prompt; give up after a while and fall back.
		err := runMSOffice(ctx, FamilyWord, "docx", src, out, tmpDir, 5*time.Minute)
		if err == nil {
			return "Microsoft Word", nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	if eng.LibreOffice != "" {
		err := runLibreOffice(ctx, eng.LibreOffice, src, out, tmpDir, "docx:MS Word 2007 XML", "--infilter=writer_pdf_import")
		if err == nil {
			return "LibreOffice", nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return i18n.L("konverter bawaan", "built-in converter"), PDFToDocx(ctx, Input{Path: src}, out, prog)
}

func isEncrypted(ctx context.Context, path string) bool {
	info, err := Inspect(ctx, path)
	return err == nil && info.Encrypted
}
