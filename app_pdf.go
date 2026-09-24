package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/pdf"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

// Input kinds of the PDF tools (file filters); every PDF task runs in queue kind "pdf".
const (
	kindPDF      = "pdf"
	kindPDFImage = "pdfimage"
	kindWord     = "word"
	kindExcel    = "excel"
	kindPPT      = "ppt"
	kindHTML     = "html"
)

var officeFamily = map[string]string{kindWord: pdf.FamilyWord, kindExcel: pdf.FamilyExcel, kindPPT: pdf.FamilyPPT}

// PdfWarmup starts the PDF engine in the background (called when the PDF page opens).
func (a *App) PdfWarmup() { pdf.Warmup() }

// PdfDoc opens a document for the page editors; password may be empty.
func (a *App) PdfDoc(path, password string) (pdf.DocInfo, error) {
	if password != "" {
		pdf.SetPassword(path, password)
	}
	info, err := pdf.Describe(context.Background(), path)
	if errors.Is(err, pdf.ErrPassword) {
		if password != "" {
			return info, errors.New(i18n.L("Password salah", "Wrong password"))
		}
		return info, errors.New("password")
	}
	return info, err
}

// PdfFind returns where a text occurs in a document (for redaction).
func (a *App) PdfFind(path, query string, matchCase bool) ([]pdf.Rect, error) {
	return pdf.FindText(context.Background(), path, query, matchCase)
}

// PdfCompare lists the differences between two documents.
func (a *App) PdfCompare(pathA, pathB string) (pdf.CompareResult, error) {
	return pdf.Compare(context.Background(), pathA, pathB)
}

// PdfOcrLanguages lists the OCR languages installed in Windows.
func (a *App) PdfOcrLanguages() ([]pdf.OCRLanguage, error) {
	return pdf.OCRLanguages(context.Background(), appdir.TempDir())
}

// PdfEnv tells the UI which helper programs the PDF tools can use.
type PdfEnv struct {
	Office  pdf.Engines `json:"office"`
	Browser string      `json:"browser"`
}

// PdfEnvironment checks Microsoft Office, LibreOffice and the browser used for web pages.
func (a *App) PdfEnvironment() PdfEnv {
	return PdfEnv{Office: pdf.DetectEngines(a.tools.Path(tools.LibreOffice)), Browser: pdf.Browser()}
}

// scanDir keeps scans and camera shots until the app restarts.
func scanDir() string {
	d := filepath.Join(appdir.TempDir(), "scans")
	_ = os.MkdirAll(d, 0o755)
	return d
}

// PdfScan shows the Windows scanner dialog; it returns nil when cancelled.
func (a *App) PdfScan() (*FileItem, error) {
	p, err := pdf.Scan(context.Background(), scanDir())
	if err != nil || p == "" {
		return nil, err
	}
	it := a.describe(kindPDFImage, p, "")
	it.Name = i18n.L("Hasil scan ", "Scan ") + it.ID + ".jpg"
	return &it, nil
}

// PdfSaveCapture stores a camera photo (data URL) as an image item.
func (a *App) PdfSaveCapture(dataURL string) (*FileItem, error) {
	data, ext, err := decodeDataURL(dataURL)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(scanDir(), fmt.Sprintf("kamera-%s.%s", nextItemID(), ext))
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return nil, err
	}
	it := a.describe(kindPDFImage, p, "")
	it.Name = i18n.L("Foto kamera ", "Camera photo ") + it.ID + "." + ext
	return &it, nil
}

func decodeDataURL(s string) ([]byte, string, error) {
	i := strings.Index(s, ",")
	if !strings.HasPrefix(s, "data:") || i < 0 {
		return nil, "", errors.New(i18n.L("data gambar tidak valid", "invalid image data"))
	}
	ext := "png"
	if strings.Contains(s[:i], "jpeg") || strings.Contains(s[:i], "jpg") {
		ext = "jpg"
	}
	data, err := base64.StdEncoding.DecodeString(s[i+1:])
	return data, ext, err
}

// PdfJob is one input of a PDF tool.
type PdfJob struct {
	ID       string `json:"id"`
	Path     string `json:"path"` // file path, or a web address for the HTML tool
	Password string `json:"password"`
}

// PdfOptions carries the settings of every PDF tool; each tool reads its own part.
type PdfOptions struct {
	Compress  pdf.CompressOptions    `json:"compress"`
	Rotate    int                    `json:"rotate"`
	Protect   pdf.ProtectOptions     `json:"protect"`
	Watermark pdf.WatermarkOptions   `json:"watermark"`
	Numbers   pdf.NumberOptions      `json:"numbers"`
	Export    pdf.ImageExportOptions `json:"export"`
	OCR       pdf.OCROptions         `json:"ocr"`
	Crop      pdf.CropOptions        `json:"crop"`
	Split     pdf.SplitOptions       `json:"split"`
	Pages     string                 `json:"pages"`    // remove / extract selection
	Separate  bool                   `json:"separate"` // extract: one file per page
	HTML      pdf.HTMLOptions        `json:"html"`
	Images    pdf.ImagesOptions      `json:"images"`
}

// tool suffixes (Indonesian, English) for results that stay PDF.
var pdfSuffix = map[string][2]string{
	"compress": {"_kecil", "_compressed"}, "repair": {"_diperbaiki", "_repaired"}, "ocr": {"_ocr", "_ocr"},
	"rotate": {"_diputar", "_rotated"}, "protect": {"_terkunci", "_protected"}, "unlock": {"_terbuka", "_unlocked"},
	"watermark": {"_watermark", "_watermark"}, "numbers": {"_bernomor", "_numbered"}, "pdfa": {"_pdfa", "_pdfa"},
	"crop": {"_dipotong", "_cropped"}, "remove": {"_dikurangi", "_pages-removed"}, "extract": {"_ekstrak", "_extracted"},
	"organize": {"_disusun", "_organized"}, "edit": {"_diedit", "_edited"}, "sign": {"_ttd", "_signed"},
	"redact": {"_disensor", "_redacted"}, "merge": {"_gabungan", "_merged"}, "split": {"_pisah", "_split"},
	"pdf2img": {"_gambar", "_images"},
}

func (a *App) suffixFor(tool string) string {
	if s, ok := pdfSuffix[tool]; ok {
		return i18n.L(s[0], s[1])
	}
	return a.cfg.Get().Suffix
}

// pdfOutput resolves where results of a source go; scans, camera shots and web pages have
// no real source folder, so they use the fixed PDF folder.
func (a *App) pdfOutput(source string) (naming.OutputSpec, error) {
	out, err := a.resolveOutput(kindPDF)
	if err != nil {
		return out, err
	}
	tmp := strings.ToLower(filepath.Clean(appdir.TempDir()))
	if out.Mode != config.OutputCustom && (source == "" || strings.HasPrefix(strings.ToLower(filepath.Clean(source)), tmp)) {
		out = naming.OutputSpec{Mode: config.OutputCustom, Dir: appdir.DefaultOutputDir(kindPDF)}
		if err := os.MkdirAll(out.Dir, 0o755); err != nil {
			return out, err
		}
	}
	return out, nil
}

// fileTask runs work into a reserved output file (temp file first, committed at the end).
func (a *App) fileTask(source string, out naming.OutputSpec, suffix, ext string, work func(ctx context.Context, tmp string, r queue.Reporter) error) queue.RunFunc {
	return func(ctx context.Context, r queue.Reporter) error {
		s := a.cfg.Get()
		target, release, err := a.namer.Reserve(source, out, suffix, ext, s.Conflict)
		if errors.Is(err, naming.ErrExists) {
			r.SetOutput(target, 0)
			return queue.Skip(i18n.L("File hasil sudah ada", "Output file already exists"))
		}
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		defer release()
		tmp := naming.TempPath(target)
		defer os.Remove(tmp)
		if err := work(ctx, tmp, r); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return pdfFail(err)
		}
		if err := naming.Commit(tmp, target); err != nil {
			return queue.Fail(i18n.L("Tidak bisa menyimpan file hasil", "Can't save the output file"), err.Error())
		}
		st, err := os.Stat(target)
		if err != nil {
			return queue.Fail(i18n.L("File hasil hilang setelah disimpan", "Output file disappeared after saving"), err.Error())
		}
		r.SetOutput(target, st.Size())
		return nil
	}
}

// folderTask runs work that writes several files. One result is kept as a single file,
// several go into their own folder.
func (a *App) folderTask(source string, out naming.OutputSpec, suffix string, work func(ctx context.Context, dir, name string, r queue.Reporter) ([]string, error)) queue.RunFunc {
	return func(ctx context.Context, r queue.Reporter) error {
		dir, err := naming.OutputDir(source, out)
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return queue.Fail(err.Error(), "")
		}
		base := naming.SanitizeFileName(strings.TrimSuffix(filepath.Base(source), filepath.Ext(source)))
		work1, err := os.MkdirTemp(dir, ".kmb-part-")
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		defer os.RemoveAll(work1)
		files, err := work(ctx, work1, base, r)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return pdfFail(err)
		}
		if len(files) == 0 {
			return queue.Fail(i18n.L("Tidak ada file yang dihasilkan", "No files were produced"), "")
		}
		if len(files) == 1 {
			ext := strings.TrimPrefix(filepath.Ext(files[0]), ".")
			target, release, err := a.namer.Reserve(source, out, suffix, ext, config.ConflictRename)
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			defer release()
			if err := os.Rename(files[0], target); err != nil {
				return queue.Fail(i18n.L("Tidak bisa menyimpan file hasil", "Can't save the output file"), err.Error())
			}
			st, _ := os.Stat(target)
			r.SetOutput(target, st.Size())
			return nil
		}
		folder := filepath.Join(dir, base+suffix)
		for k := 2; exists(folder); k++ {
			folder = filepath.Join(dir, fmt.Sprintf("%s%s (%d)", base, suffix, k))
		}
		if err := os.Rename(work1, folder); err != nil {
			return queue.Fail(i18n.L("Tidak bisa menyimpan folder hasil", "Can't save the output folder"), err.Error())
		}
		var total int64
		for _, f := range files {
			if st, err := os.Stat(filepath.Join(folder, filepath.Base(f))); err == nil {
				total += st.Size()
			}
		}
		r.SetOutput(folder, total)
		r.Message(fmt.Sprintf(i18n.L("%d file", "%d files"), len(files)))
		return nil
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func pdfFail(err error) error {
	var ue *queue.UserError
	var se *queue.SkipError
	if errors.As(err, &ue) || errors.As(err, &se) {
		return err
	}
	if errors.Is(err, pdf.ErrPassword) {
		return queue.Fail(i18n.L("PDF dikunci password — isi passwordnya", "The PDF is password protected — enter its password"), err.Error())
	}
	if errors.Is(err, pdf.ErrNoOffice) {
		return queue.Fail(i18n.L("Butuh Microsoft Office atau LibreOffice — pasang LibreOffice di Pengaturan", "Needs Microsoft Office or LibreOffice — install LibreOffice in Settings"), "")
	}
	msg := err.Error()
	if r := []rune(msg); len(r) > 0 {
		msg = strings.ToUpper(string(r[:1])) + string(r[1:])
	}
	return queue.Fail(strings.SplitN(msg, "\n", 2)[0], err.Error())
}

func progress(r queue.Reporter) pdf.Progress { return func(p float64) { r.Progress(p) } }

var pdfVerb = map[string][2]string{
	"compress": {"Mengompres…", "Compressing…"}, "repair": {"Memperbaiki…", "Repairing…"}, "ocr": {"Mengenali teks…", "Recognising text…"},
	"protect": {"Mengunci…", "Locking…"}, "unlock": {"Membuka kunci…", "Unlocking…"}, "split": {"Memisahkan…", "Splitting…"},
	"merge": {"Menggabungkan…", "Merging…"}, "html": {"Membuka halaman…", "Loading the page…"}, "redact": {"Menyensor…", "Redacting…"},
}

func verb(tool string) string {
	if v, ok := pdfVerb[tool]; ok {
		return i18n.L(v[0], v[1])
	}
	return i18n.L("Memproses…", "Processing…")
}

// StartPdf queues a per-file PDF tool.
func (a *App) StartPdf(tool string, items []PdfJob, o PdfOptions) ([]JobRef, error) {
	if len(items) == 0 {
		return nil, errors.New(i18n.L("Tambahkan file dulu", "Add files first"))
	}
	if _, err := a.resolveOutput(kindPDF); err != nil {
		return nil, err
	}
	env := a.PdfEnvironment()
	switch tool {
	case "word2pdf", "excel2pdf", "ppt2pdf":
		if !env.Office.Word && !env.Office.Excel && !env.Office.PowerPoint && env.Office.LibreOffice == "" {
			return nil, errors.New(i18n.L("Konversi dokumen Office butuh Microsoft Office atau LibreOffice. Pasang LibreOffice di Pengaturan › Tools pendukung.", "Office conversion needs Microsoft Office or LibreOffice. Install LibreOffice in Settings › Helper tools."))
		}
	case "html":
		if env.Browser == "" {
			return nil, errors.New(i18n.L("Microsoft Edge atau Google Chrome tidak ditemukan", "Microsoft Edge or Google Chrome was not found"))
		}
	}
	tmpDir := appdir.TempDir()
	specs := make([]queue.Spec, 0, len(items))
	for _, it := range items {
		it := it
		in := pdf.Input{Path: it.Path, Password: it.Password}
		title := filepath.Base(it.Path)
		source := it.Path
		if tool == "html" {
			if u, err := pdf.NormalizeURL(it.Path); err == nil && !strings.HasPrefix(u, "file:") {
				title = it.Path
				source = ""
			}
		}
		out, err := a.pdfOutput(source)
		if err != nil {
			return nil, err
		}
		if source == "" {
			source = filepath.Join(out.Dir, pdf.URLName(it.Path)+".html")
		}
		if tool != "html" {
			if _, err := os.Stat(it.Path); err != nil {
				return nil, fmt.Errorf(i18n.L("File tidak ditemukan: %s", "File not found: %s"), it.Path)
			}
		}
		run, err := a.pdfRun(tool, in, source, out, o, env, tmpDir)
		if err != nil {
			return nil, err
		}
		specs = append(specs, queue.Spec{Title: title, Run: func(ctx context.Context, r queue.Reporter) error {
			r.Message(verb(tool))
			return run(ctx, r)
		}})
	}
	ids := a.queue.AddMany(queue.KindPDF, specs)
	refs := make([]JobRef, len(items))
	for i := range items {
		refs[i] = JobRef{ItemID: items[i].ID, TaskID: ids[i]}
	}
	return refs, nil
}

func (a *App) pdfRun(tool string, in pdf.Input, source string, out naming.OutputSpec, o PdfOptions, env PdfEnv, tmpDir string) (queue.RunFunc, error) {
	suffix := a.suffixFor(tool)
	same := func(work func(ctx context.Context, tmp string, r queue.Reporter) error) queue.RunFunc {
		return a.fileTask(source, out, suffix, "pdf", work)
	}
	switch tool {
	case "compress":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			err := pdf.Compress(ctx, in, o.Compress, tmp, progress(r))
			if errors.Is(err, pdf.ErrNoGain) {
				return queue.Skip(i18n.L("Sudah optimal, tidak bisa diperkecil lagi", "Already optimal, can't get smaller"))
			}
			return err
		}), nil
	case "repair":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error { return pdf.Repair(ctx, in, tmp, tmpDir) }), nil
	case "ocr":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			n, err := pdf.OCR(ctx, in, o.OCR, tmp, tmpDir, progress(r))
			if err == nil {
				r.Message(fmt.Sprintf(i18n.L("%d halaman dikenali", "%d pages recognised"), n))
			}
			return err
		}), nil
	case "rotate":
		deg := ((o.Rotate % 360) + 360) % 360
		if deg == 0 || deg%90 != 0 {
			return nil, errors.New(i18n.L("Pilih arah putar", "Choose a rotation"))
		}
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.Rotate(in, deg, o.Pages, tmp)
		}), nil
	case "protect":
		if o.Protect.Password == "" {
			return nil, errors.New(i18n.L("Isi password", "Enter a password"))
		}
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.Protect(in, o.Protect, tmp, tmpDir)
		}), nil
	case "unlock":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error { return pdf.Unlock(in, tmp) }), nil
	case "watermark":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.Watermark(in, o.Watermark, tmp)
		}), nil
	case "numbers":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.AddPageNumbers(in, o.Numbers, tmp)
		}), nil
	case "pdfa":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			res, err := pdf.ToPDFA(ctx, in, tmp, progress(r))
			if err == nil && len(res.RasterPages) > 0 {
				r.Message(fmt.Sprintf(i18n.L("%d halaman dijadikan gambar (font tidak tertanam)", "%d pages became images (fonts not embedded)"), len(res.RasterPages)))
			}
			return err
		}), nil
	case "crop":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error { return pdf.Crop(in, o.Crop, tmp) }), nil
	case "remove":
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.RemovePages(in, o.Pages, tmp)
		}), nil
	case "extract":
		if o.Separate {
			return a.folderTask(source, out, suffix, func(ctx context.Context, dir, name string, r queue.Reporter) ([]string, error) {
				return pdf.ExtractPagesSeparate(ctx, in, o.Pages, dir, name, progress(r))
			}), nil
		}
		return same(func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.ExtractPages(in, o.Pages, tmp)
		}), nil
	case "split":
		return a.folderTask(source, out, suffix, func(ctx context.Context, dir, name string, r queue.Reporter) ([]string, error) {
			return pdf.Split(ctx, in, o.Split, dir, name, progress(r))
		}), nil
	case "pdf2img":
		return a.folderTask(source, out, suffix, func(ctx context.Context, dir, name string, r queue.Reporter) ([]string, error) {
			return pdf.ToImages(ctx, in, o.Export, dir, name, progress(r))
		}), nil
	case "pdf2word":
		return a.fileTask(source, out, suffix, "docx", func(ctx context.Context, tmp string, r queue.Reporter) error {
			used, err := pdf.PDFToWord(ctx, in, tmp, tmpDir, env.Office, progress(r))
			if err == nil {
				r.Message(i18n.L("Dengan ", "With ") + used)
			}
			return err
		}), nil
	case "pdf2ppt":
		return a.fileTask(source, out, suffix, "pptx", func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.PDFToPptx(ctx, in, tmp, progress(r))
		}), nil
	case "pdf2excel":
		return a.fileTask(source, out, suffix, "xlsx", func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.PDFToXlsx(ctx, in, tmp, progress(r))
		}), nil
	case "word2pdf", "excel2pdf", "ppt2pdf":
		return a.fileTask(source, out, suffix, "pdf", func(ctx context.Context, tmp string, r queue.Reporter) error {
			r.Progress(-1)
			used, err := pdf.OfficeToPDF(ctx, in.Path, tmp, tmpDir, env.Office)
			if err == nil {
				r.Message(i18n.L("Dengan ", "With ") + used)
			}
			return err
		}), nil
	case "img2pdf":
		return a.fileTask(source, out, suffix, "pdf", func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.ImagesToPDF(ctx, []string{in.Path}, tmp, o.Images, a.tools.Path(tools.FFmpeg), progress(r))
		}), nil
	case "html":
		return a.fileTask(source, out, suffix, "pdf", func(ctx context.Context, tmp string, r queue.Reporter) error {
			r.Progress(-1)
			return pdf.HTMLToPDF(ctx, in.Path, o.HTML, tmp, tmpDir)
		}), nil
	}
	return nil, fmt.Errorf(i18n.L("alat tidak dikenal: %s", "unknown tool: %s"), tool)
}

// StartPdfCombine makes one PDF from several inputs (merge, images → PDF, scans).
func (a *App) StartPdfCombine(tool string, items []PdfJob, o PdfOptions) (JobRef, error) {
	if len(items) == 0 {
		return JobRef{}, errors.New(i18n.L("Tambahkan file dulu", "Add files first"))
	}
	first := items[0].Path
	out, err := a.pdfOutput(first)
	if err != nil {
		return JobRef{}, err
	}
	tmpDir := appdir.TempDir()
	suffix := a.suffixFor(tool)
	source := first
	if tool != "merge" {
		suffix = ""
		source = filepath.Join(filepath.Dir(first), naming.SanitizeFileName(strings.TrimSuffix(filepath.Base(first), filepath.Ext(first))))
	}
	var run queue.RunFunc
	switch tool {
	case "merge":
		inputs := make([]pdf.Input, len(items))
		for i, it := range items {
			inputs[i] = pdf.Input{Path: it.Path, Password: it.Password}
		}
		run = a.fileTask(source, out, suffix, "pdf", func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.Merge(ctx, inputs, tmp, tmpDir, progress(r))
		})
	case "img2pdf", "scan":
		files := make([]string, len(items))
		for i, it := range items {
			files[i] = it.Path
		}
		if tool == "scan" {
			source = filepath.Join(out.Dir, i18n.L("Hasil scan", "Scan"))
			if out.Dir == "" {
				source = filepath.Join(appdir.DefaultOutputDir(kindPDF), i18n.L("Hasil scan", "Scan"))
			}
		}
		run = a.fileTask(source, out, suffix, "pdf", func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.ImagesToPDF(ctx, files, tmp, o.Images, a.tools.Path(tools.FFmpeg), progress(r))
		})
	default:
		return JobRef{}, fmt.Errorf(i18n.L("alat tidak dikenal: %s", "unknown tool: %s"), tool)
	}
	title := fmt.Sprintf("%s (+%d)", filepath.Base(first), len(items)-1)
	id := a.queue.Add(queue.KindPDF, title, func(ctx context.Context, r queue.Reporter) error {
		r.Message(verb(tool))
		return run(ctx, r)
	})
	return JobRef{ItemID: "combine", TaskID: id}, nil
}

// EditItem is an object placed on a page in the editor (points, display space).
type EditItem struct {
	pdf.Item
	Page     int    `json:"page"`     // 1-based
	ImageURL string `json:"imageUrl"` // data: URL for pictures and signatures
}

// PdfEditRequest describes a page-editor job.
type PdfEditRequest struct {
	Tool    string            `json:"tool"` // organize | edit | sign | redact | crop
	Sources []pdf.Input       `json:"sources"`
	Pages   []pdf.PageRef     `json:"pages"`
	Items   []EditItem        `json:"items"`
	Redact  pdf.RedactOptions `json:"redact"`
	Crop    pdf.CropOptions   `json:"crop"`
}

// StartPdfEdit runs a page-editor job (organise, edit, sign, redact, crop).
func (a *App) StartPdfEdit(req PdfEditRequest) (JobRef, error) {
	if len(req.Sources) == 0 {
		return JobRef{}, errors.New(i18n.L("Buka PDF dulu", "Open a PDF first"))
	}
	src := req.Sources[0]
	out, err := a.pdfOutput(src.Path)
	if err != nil {
		return JobRef{}, err
	}
	tmpDir := appdir.TempDir()
	suffix := a.suffixFor(req.Tool)
	var work func(ctx context.Context, tmp string, r queue.Reporter) error
	switch req.Tool {
	case "organize":
		work = func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.Organize(ctx, req.Sources, req.Pages, tmp, tmpDir, progress(r))
		}
	case "edit", "sign":
		if len(req.Items) == 0 {
			return JobRef{}, errors.New(i18n.L("Belum ada yang ditambahkan ke halaman", "Nothing has been added to the pages yet"))
		}
		byPage := map[int][]pdf.Item{}
		for _, it := range req.Items {
			item := it.Item
			if it.ImageURL != "" {
				data, _, err := decodeDataURL(it.ImageURL)
				if err != nil {
					return JobRef{}, err
				}
				item.Image = data
			}
			byPage[it.Page] = append(byPage[it.Page], item)
		}
		work = func(ctx context.Context, tmp string, r queue.Reporter) error { return pdf.DrawItems(src, byPage, tmp) }
	case "redact":
		work = func(ctx context.Context, tmp string, r queue.Reporter) error {
			return pdf.Redact(ctx, src, req.Redact, tmp, progress(r))
		}
	case "crop":
		work = func(ctx context.Context, tmp string, r queue.Reporter) error { return pdf.Crop(src, req.Crop, tmp) }
	default:
		return JobRef{}, fmt.Errorf(i18n.L("alat tidak dikenal: %s", "unknown tool: %s"), req.Tool)
	}
	run := a.fileTask(src.Path, out, suffix, "pdf", work)
	id := a.queue.Add(queue.KindPDF, filepath.Base(src.Path), func(ctx context.Context, r queue.Reporter) error {
		r.Message(verb(req.Tool))
		return run(ctx, r)
	})
	return JobRef{ItemID: "edit", TaskID: id}, nil
}
