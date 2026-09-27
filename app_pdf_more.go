package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/pdf"
	"kuymediabox/internal/queue"
)

// PDF tools with their own editor: document properties & bookmarks, and form filling.

// PdfDetails reads a document's properties and bookmarks.
func (a *App) PdfDetails(path, password string) (pdf.DocDetails, error) {
	return pdf.ReadDetails(pdf.Input{Path: path, Password: password})
}

// PdfFormFields lists the fillable fields of a PDF form (none for a PDF without a form).
func (a *App) PdfFormFields(path, password string) ([]pdf.FormField, error) {
	fields, err := pdf.ReadForm(pdf.Input{Path: path, Password: password})
	if errors.As(err, new(pdf.NoFormError)) {
		return []pdf.FormField{}, nil
	}
	return fields, err
}

// PdfTSAServers lists public time-stamping servers for the digital signature.
func (a *App) PdfTSAServers() []string { return pdf.TSAServers }

// StartPdfMeta saves new properties and bookmarks into a copy of a PDF. keepBookmarks
// leaves the existing bookmarks alone.
func (a *App) StartPdfMeta(item PdfJob, meta pdf.Meta, bookmarks []pdf.Bookmark, keepBookmarks bool) (JobRef, error) {
	if keepBookmarks {
		bookmarks = nil
	} else if bookmarks == nil {
		bookmarks = []pdf.Bookmark{}
	}
	return a.singlePdfTask("meta", i18n.L("_properti", "_properties"), item, func(ctx context.Context, in pdf.Input, tmp string) error {
		return pdf.WriteDetails(in, meta, bookmarks, tmp)
	})
}

// StartPdfForm fills a PDF form; flatten makes the result non-editable.
func (a *App) StartPdfForm(item PdfJob, fields []pdf.FormField, flatten bool) (JobRef, error) {
	suffix := i18n.L("_terisi", "_filled")
	return a.singlePdfTask("form", suffix, item, func(ctx context.Context, in pdf.Input, tmp string) error {
		return pdf.FillForm(ctx, in, fields, flatten, tmp)
	})
}

func (a *App) singlePdfTask(tool, suffix string, item PdfJob, work func(ctx context.Context, in pdf.Input, tmp string) error) (JobRef, error) {
	if _, err := os.Stat(item.Path); err != nil {
		return JobRef{}, errors.New(i18n.L("File tidak ditemukan", "File not found"))
	}
	out, err := a.pdfOutput(item.Path)
	if err != nil {
		return JobRef{}, err
	}
	in := pdf.Input{Path: item.Path, Password: item.Password}
	run := a.fileTask(item.Path, out, suffix, "pdf", func(ctx context.Context, tmp string, r queue.Reporter) error {
		r.Message(verb(tool))
		return work(ctx, in, tmp)
	})
	id := a.queue.Add(queue.KindPDF, filepath.Base(item.Path), run)
	return JobRef{ItemID: item.ID, TaskID: id}, nil
}
