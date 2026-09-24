package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTMLToPDF(t *testing.T) {
	if Browser() == "" {
		t.Skip("no Edge/Chrome")
	}
	dir := t.TempDir()
	page := filepath.Join(dir, "halaman.html")
	body := "<html><body style='background:#fde'><h1>Judul Uji</h1>" + strings.Repeat("<p>Paragraf panjang untuk mengisi halaman.</p>", 120) + "</body></html>"
	_ = os.WriteFile(page, []byte(body), 0o644)
	bg := context.Background()
	out := filepath.Join(dir, "a.pdf")
	if err := HTMLToPDF(bg, page, HTMLOptions{PageSize: "a4", Margin: "normal", Background: true}, out, dir); err != nil {
		t.Fatal(err)
	}
	info, err := Inspect(bg, out)
	if err != nil || info.Pages < 2 || info.Width < 590 || info.Width > 600 {
		t.Fatalf("paged: %+v %v", info, err)
	}
	one := filepath.Join(dir, "one.pdf")
	if err := HTMLToPDF(bg, page, HTMLOptions{OnePage: true, Width: 1024}, one, dir); err != nil {
		t.Fatal(err)
	}
	if info, _ := Inspect(bg, one); info.Pages != 1 {
		t.Fatalf("one page: %+v", info)
	}
	d, _ := Open(bg, out, "")
	txt, _ := d.PageText(0)
	d.Close()
	if !strings.Contains(txt, "Judul Uji") {
		t.Fatalf("text %q", txt)
	}
	if n := URLName("https://www.contoh.co.id/berita/hari-ini?x=1"); n != "contoh-co-id-berita-hari-ini" {
		t.Fatalf("name %q", n)
	}
	if os.Getenv("KMB_NET_TEST") != "" {
		if err := HTMLToPDF(bg, "example.com", HTMLOptions{}, filepath.Join(dir, "web.pdf"), dir); err != nil {
			t.Fatal(err)
		}
	}
}
