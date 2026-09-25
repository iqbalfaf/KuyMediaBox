package pdf

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestDigitalSignature(t *testing.T) {
	dir := t.TempDir()
	src := makeTextPDF(t, dir, "doc.pdf", 2)
	p12 := filepath.Join(dir, "me.p12")
	if err := CreateCertificate("Budi Santoso", "budi@example.com", "KMB", 2, "rahasia", p12); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSigner(p12, "salah"); err == nil {
		t.Fatal("wrong password accepted")
	}
	s, err := LoadSigner(p12, "rahasia")
	if err != nil {
		t.Fatal(err)
	}
	if info := s.Describe(); info.Name != "Budi Santoso" || !info.SelfSign || info.Email != "budi@example.com" {
		t.Fatalf("describe %+v", info)
	}
	// Sign with a certificate issued by a test CA that the validator trusts.
	s, caDER := caSigner(t)
	certDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(certDir, "ca.crt"), caDER, 0o644); err != nil {
		t.Fatal(err)
	}
	orig := model.TrustedCertDir
	model.TrustedCertDir = certDir
	defer func() { model.TrustedCertDir = orig }()
	for _, visible := range []bool{false, true} {
		out := filepath.Join(dir, "signed.pdf")
		o := DigitalSignOptions{Reason: "Persetujuan", Location: "Jakarta", Visible: visible, Position: "br", Page: "last"}
		if err := SignDigital(Input{Path: src}, o, s, out); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(out)
		if !bytes.Contains(data, []byte("/adbe.pkcs7.detached")) {
			t.Fatal("no signature dict")
		}
		results, err := api.ValidateSignatures(out, true, model.NewDefaultConfiguration())
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("%d signatures", len(results))
		}
		r := results[0]
		t.Logf("visible=%v status=%v reason=%v problems=%v", visible, r.Status, r.Reason, r.Problems)
		// Revocation can't be checked for a test CA; integrity and trust must pass.
		if r.Status == model.SignatureStatusInvalid || (r.Status != model.SignatureStatusValid && r.Reason != model.SignatureReasonCertRevocationUnknown) {
			t.Fatalf("signature not intact: %v %v", r.Reason, r.Problems)
		}
		// The signed file still opens and keeps its pages.
		ctx, err := readCtx(out, "")
		if err != nil || ctx.PageCount != 2 {
			t.Fatalf("reopen: %v", err)
		}
		// Changing a signed byte must break the signature.
		i := bytes.Index(data, []byte("Laporan"))
		if i < 0 {
			i = 200
		}
		data[i+1] ^= 0x01
		bad := filepath.Join(dir, "tampered.pdf")
		_ = os.WriteFile(bad, data, 0o644)
		res, err := api.ValidateSignatures(bad, true, model.NewDefaultConfiguration())
		if err == nil && len(res) == 1 && res[0].Status != model.SignatureStatusInvalid {
			t.Fatalf("tampered file not detected: %v %v", res[0].Status, res[0].Reason)
		}
	}
}

func caSigner(t *testing.T) (*Signer, []byte) {
	t.Helper()
	caKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	caTpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "KMB Test CA"}, NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().AddDate(1, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(caDER)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Budi"}, NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment}
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	return &Signer{Key: key, Cert: cert, Chain: []*x509.Certificate{ca}}, caDER
}
