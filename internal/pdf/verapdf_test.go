package pdf

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Runs only when veraPDF is available: KMB_VERAPDF=<verapdf.bat>, KMB_JAVA=<java.exe>.
func TestValidatePDFA(t *testing.T) {
	bat, java := os.Getenv("KMB_VERAPDF"), os.Getenv("KMB_JAVA")
	if bat == "" || java == "" {
		t.Skip("veraPDF not configured")
	}
	dir := t.TempDir()
	src := makeTextPDF(t, dir, "plain.pdf", 1)
	pdfa := filepath.Join(dir, "a.pdf")
	if _, err := ToPDFA(context.Background(), Input{Path: src}, pdfa, func(float64) {}); err != nil {
		t.Fatal(err)
	}
	res, err := ValidatePDFA(context.Background(), bat, java, pdfa, "0")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Compliant || res.ShortProfile() != "PDF/A-2b" {
		t.Fatalf("converted file: %+v", res)
	}
	res, err = ValidatePDFA(context.Background(), bat, java, src, "2b")
	if err != nil {
		t.Fatal(err)
	}
	if res.Compliant || len(res.Failed) == 0 {
		t.Fatalf("plain file should fail: %+v", res)
	}
	t.Log(res.Report())
}
