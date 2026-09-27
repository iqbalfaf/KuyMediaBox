//go:build integration

package pdf

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestTimestampLive signs with a time stamp from a public TSA and checks the token is
// embedded as an unsigned attribute.
func TestTimestampLive(t *testing.T) {
	dir := t.TempDir()
	src := makeTextPDF(t, dir, "doc.pdf", 1)
	p12 := filepath.Join(dir, "me.p12")
	if err := CreateCertificate("Budi", "b@example.com", "KMB", 1, "rahasia", p12); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSigner(p12, "rahasia")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "signed.pdf")
	if err := SignDigital(Input{Path: src}, DigitalSignOptions{TSA: "timestamp.digicert.com"}, s, out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	i := bytes.Index(data, []byte("/Contents <")) + len("/Contents <")
	j := bytes.IndexByte(data[i:], '>') + i
	blob, err := hex.DecodeString(string(bytes.TrimRight(data[i:j], "0")) + map[bool]string{true: "0", false: ""}[len(bytes.TrimRight(data[i:j], "0"))%2 == 1])
	if err != nil {
		t.Fatal(err)
	}
	oid := mustMarshal(oidTimeStampToken)
	if !bytes.Contains(blob, oid) {
		t.Fatal("no time-stamp token in the signature")
	}
	_ = os.WriteFile(filepath.Join("C:/Users/iqbal/AppData/Local/Temp/claude/d--VIBE-CODING-KuyMediaBox/8914db72-9e97-4191-a67f-a73829d26cf0/scratchpad", "sig.der"), blob, 0o644)
}
