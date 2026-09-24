package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>tag:github.com,2008:Repository/1/v0.3.0</id>
    <updated>2026-10-01T10:00:00Z</updated>
    <link rel="alternate" type="text/html" href="https://github.com/iqbalfaf/KuyMediaBox/releases/tag/v0.3.0"/>
    <content type="html">&lt;h2&gt;Yang baru&lt;/h2&gt;
&lt;ul&gt;
&lt;li&gt;Bitrate audio bisa dipilih &amp;amp; format WAV&lt;/li&gt;
&lt;li&gt;Cek update tetap jalan saat &lt;code&gt;API&lt;/code&gt; dibatasi&lt;/li&gt;
&lt;/ul&gt;
&lt;h2&gt;Unduh&lt;/h2&gt;
&lt;ul&gt;
&lt;li&gt;setup.exe: installer&lt;/li&gt;
&lt;/ul&gt;</content>
  </entry>
</feed>`

func TestCheckFallsBackWhenRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"):
			http.Error(w, "rate limited", http.StatusForbidden)
		case r.URL.Path == "/"+Repo+"/releases/latest":
			http.Redirect(w, r, "/"+Repo+"/releases/tag/v0.3.0", http.StatusFound)
		case r.URL.Path == "/"+Repo+"/releases.atom":
			_, _ = w.Write([]byte(feed))
		case strings.HasPrefix(r.URL.Path, "/"+Repo+"/releases/download/v0.3.0/KuyMediaBox-v0.3.0-windows-x64-"):
			w.Header().Set("Content-Length", "1234")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	APIBase, WebBase = srv.URL, srv.URL
	defer func() { APIBase, WebBase = "https://api.github.com", "https://github.com" }()

	info, err := Check(context.Background(), "0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "0.3.0" || info.AssetSize != 1234 || info.sumsURL == "" {
		t.Fatalf("info %+v", info)
	}
	if !strings.HasSuffix(info.assetURL, "/releases/download/v0.3.0/"+info.AssetName) {
		t.Fatalf("asset url %s", info.assetURL)
	}
	want := "- Bitrate audio bisa dipilih & format WAV\n- Cek update tetap jalan saat API dibatasi"
	if info.Notes != want || info.PublishedAt != "2026-10-01T10:00:00Z" {
		t.Fatalf("notes %q / %q", info.Notes, info.PublishedAt)
	}
}

// Run against the real GitHub with KMB_LIVE=1.
func TestLiveCheck(t *testing.T) {
	if os.Getenv("KMB_LIVE") == "" {
		t.Skip()
	}
	info, err := checkWeb(context.Background(), "0.1.0", ModePortable)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s available=%v asset=%s size=%d\nnotes:\n%s", info.Latest, info.Available, info.AssetName, info.AssetSize, info.Notes)
	if !info.Available || info.AssetSize == 0 {
		t.Fatalf("live: %+v", info)
	}
}
