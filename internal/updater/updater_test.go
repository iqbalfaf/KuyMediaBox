package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.1", "0.1.0", 1},
		{"0.1.0", "0.1.0", 0},
		{"0.2.0", "0.10.0", -1},
		{"1.0", "0.9.9", 1},
		{"0.1.0", "0.1.0-dev", 0},
		{"v1.2.3", "1.2.3", 0},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func release(tag string) ghRelease {
	return ghRelease{
		TagName: tag,
		Body:    "## Unduh\n- file\n\n## What's Changed\n* Fitur baru",
		Assets: []ghAsset{
			{Name: "KuyMediaBox-" + tag + "-windows-x64-portable.exe", URL: "https://x/p.exe", Size: 10},
			{Name: "KuyMediaBox-" + tag + "-windows-x64-setup.exe", URL: "https://x/s.exe", Size: 5},
			{Name: "SHA256SUMS.txt", URL: "https://x/sums"},
		},
	}
}

func TestFromRelease(t *testing.T) {
	info := fromRelease(release("v0.2.0"), "0.1.0", ModePortable)
	if !info.Available || info.Latest != "0.2.0" || !strings.HasSuffix(info.AssetName, "portable.exe") {
		t.Fatalf("portable: %+v", info)
	}
	if info.Notes != "## What's Changed\n* Fitur baru" {
		t.Fatalf("notes %q", info.Notes)
	}
	inst := fromRelease(release("v0.2.0"), "0.1.0", ModeInstaller)
	if !strings.HasSuffix(inst.AssetName, "setup.exe") {
		t.Fatalf("installer asset: %s", inst.AssetName)
	}
	same := fromRelease(release("v0.1.0"), "0.1.0", ModePortable)
	if same.Available {
		t.Fatal("same version must not be an update")
	}
	pre := release("v0.3.0")
	pre.Prerelease = true
	if fromRelease(pre, "0.1.0", ModePortable).Available {
		t.Fatal("pre-releases must be ignored")
	}
}

func TestParseSums(t *testing.T) {
	bom := string(rune(0xFEFF))
	sums := bom + strings.Repeat("c", 64) + "  other.exe\n" + strings.Repeat("b", 64) + "  KuyMediaBox-v1-windows-x64-portable.exe\r\n"
	h, err := parseSums(strings.NewReader(sums), "KuyMediaBox-v1-windows-x64-portable.exe")
	if err != nil || h != strings.Repeat("b", 64) {
		t.Fatalf("%q %v", h, err)
	}
	if _, err := parseSums(strings.NewReader(sums), "missing.exe"); err == nil {
		t.Fatal("expected error for missing entry")
	}
}

func TestCheckAndDownloadVerifiesChecksum(t *testing.T) {
	payload := []byte("versi baru KuyMediaBox")
	sum := sha256.Sum256(payload)
	good := hex.EncodeToString(sum[:])
	var sums string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + Repo + "/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v9.9.9","html_url":"u","body":"b","assets":[
				{"name":"KuyMediaBox-v9.9.9-windows-x64-portable.exe","browser_download_url":"%[1]s/p.exe","size":%[2]d},
				{"name":"KuyMediaBox-v9.9.9-windows-x64-setup.exe","browser_download_url":"%[1]s/s.exe","size":%[2]d},
				{"name":"SHA256SUMS.txt","browser_download_url":"%[1]s/sums"}]}`, srv.URL, len(payload))
		case "/p.exe", "/s.exe":
			w.Write(payload)
		case "/sums":
			fmt.Fprint(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := APIBase
	APIBase = srv.URL
	defer func() { APIBase = old }()

	info, err := Check(context.Background(), "0.1.0")
	if err != nil || !info.Available || info.Latest != "9.9.9" {
		t.Fatalf("check: %+v %v", info, err)
	}
	dir := t.TempDir()

	sums = good + "  KuyMediaBox-v9.9.9-windows-x64-portable.exe\n" + good + "  KuyMediaBox-v9.9.9-windows-x64-setup.exe\n"
	var last float64
	path, err := Download(context.Background(), info, dir, func(p float64) { last = p })
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(payload) || last != 1 || filepath.Dir(path) != dir {
		t.Fatalf("download result wrong: %q progress=%v", data, last)
	}

	sums = strings.Repeat("0", 64) + "  " + info.AssetName + "\n"
	if _, err := Download(context.Background(), info, t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered file must be rejected, got %v", err)
	}
}

func TestParseSumsFirstLineWithBOM(t *testing.T) {
	bom := string(rune(0xFEFF))
	h, err := parseSums(strings.NewReader(bom+strings.Repeat("a", 64)+"  first.exe\n"), "first.exe")
	if err != nil || h != strings.Repeat("a", 64) {
		t.Fatalf("BOM on the first line must be ignored: %q %v", h, err)
	}
}
