package pdf

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"software.sslmate.com/src/go-pkcs12"

	"kuymediabox/internal/i18n"
)

// DigitalSignOptions describe a certificate-based (PKCS#7) signature.
type DigitalSignOptions struct {
	CertFile     string `json:"certFile"`     // .pfx / .p12
	CertPassword string `json:"certPassword"` // never stored
	Name         string `json:"name"`         // shown signer name ("" = from the certificate)
	Reason       string `json:"reason"`
	Location     string `json:"location"`
	Contact      string `json:"contact"`
	Visible      bool   `json:"visible"`  // draw a small stamp on the page
	Position     string `json:"position"` // bl br tl tr
	Page         string `json:"page"`     // first | last
}

// Signer is a loaded certificate with its private key.
type Signer struct {
	Key   crypto.Signer
	Cert  *x509.Certificate
	Chain []*x509.Certificate
}

// LoadSigner reads a .pfx/.p12 file.
func LoadSigner(path, password string) (*Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.L("file sertifikat tidak bisa dibaca: %w", "the certificate file can't be read: %w"), err)
	}
	key, cert, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		if errors.Is(err, pkcs12.ErrIncorrectPassword) {
			return nil, errors.New(i18n.L("Password sertifikat salah", "Wrong certificate password"))
		}
		return nil, fmt.Errorf(i18n.L("sertifikat tidak bisa dibuka: %w", "the certificate can't be opened: %w"), err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New(i18n.L("jenis kunci sertifikat tidak didukung", "unsupported certificate key type"))
	}
	switch signer.(type) {
	case *rsa.PrivateKey, *ecdsa.PrivateKey:
	default:
		return nil, errors.New(i18n.L("hanya kunci RSA atau ECDSA yang didukung", "only RSA or ECDSA keys are supported"))
	}
	now := time.Now()
	if now.After(cert.NotAfter) {
		return nil, fmt.Errorf(i18n.L("sertifikat sudah kedaluwarsa (%s)", "the certificate expired on %s"), cert.NotAfter.Format("2006-01-02"))
	}
	return &Signer{Key: signer, Cert: cert, Chain: chain}, nil
}

// CertInfo describes a certificate for the UI.
type CertInfo struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Issuer    string `json:"issuer"`
	SelfSign  bool   `json:"selfSigned"`
	NotBefore string `json:"notBefore"`
	NotAfter  string `json:"notAfter"`
}

// Describe returns the certificate's details.
func (s *Signer) Describe() CertInfo {
	c := s.Cert
	info := CertInfo{Name: c.Subject.CommonName, Issuer: c.Issuer.CommonName, NotBefore: c.NotBefore.Format("2006-01-02"), NotAfter: c.NotAfter.Format("2006-01-02")}
	if len(c.EmailAddresses) > 0 {
		info.Email = c.EmailAddresses[0]
	}
	info.SelfSign = bytes.Equal(c.RawIssuer, c.RawSubject)
	return info
}

var oidDocumentSigning = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 36}

// CreateCertificate makes a self-signed signing certificate and saves it as a .p12 file.
func CreateCertificate(name, email, org string, years int, password, out string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New(i18n.L("Isi nama pemilik sertifikat", "Enter the certificate owner's name"))
	}
	if len(password) < 4 {
		return errors.New(i18n.L("Password sertifikat minimal 4 karakter", "The certificate password needs at least 4 characters"))
	}
	if years < 1 || years > 20 {
		years = 5
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return err
	}
	subject := pkix.Name{CommonName: name}
	if org = strings.TrimSpace(org); org != "" {
		subject.Organization = []string{org}
	}
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               subject,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(years, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		UnknownExtKeyUsage:    []asn1.ObjectIdentifier{oidDocumentSigning},
		BasicConstraintsValid: true,
	}
	if email = strings.TrimSpace(email); email != "" {
		tpl.EmailAddresses = []string{email}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	pfx, err := pkcs12.Modern.Encode(key, cert, nil, password)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, pfx, 0o600)
}

// sigSize is the room reserved for the PKCS#7 blob (bytes, hex doubles it).
const sigSize = 24 * 1024

// SignDigital adds a certificate signature (adbe.pkcs7.detached) as an incremental update.
// The document is first rewritten without encryption and with a classic xref table, so
// earlier digital signatures of the source do not survive.
func SignDigital(in Input, o DigitalSignOptions, s *Signer, out string) error {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	ctx.WriteObjectStream = false
	ctx.WriteXRefStream = false
	base := out + ".base"
	defer os.Remove(base)
	if err := writeCtx(ctx, base); err != nil {
		return err
	}
	data, err := os.ReadFile(base)
	if err != nil {
		return err
	}
	ctx, err = readCtx(base, "")
	if err != nil {
		return err
	}
	prev, err := lastStartXref(data)
	if err != nil {
		return err
	}
	root := ctx.XRefTable.Root
	if root == nil {
		return errors.New("PDF catalog missing")
	}
	catalog, err := ctx.DereferenceDict(*root)
	if err != nil || catalog == nil {
		return errors.New("PDF catalog unreadable")
	}
	pageNr := 1
	if o.Page == "last" {
		pageNr = ctx.PageCount
	}
	pageDict, pageRef, _, err := ctx.PageDict(pageNr, false)
	if err != nil || pageDict == nil || pageRef == nil {
		return fmt.Errorf("page %d unreadable", pageNr)
	}
	_, geom, err := pageGeom(ctx, pageNr)
	if err != nil {
		return err
	}

	next := *ctx.XRefTable.Size
	alloc := func() int { next++; return next - 1 }
	var objs []pdfObj

	signerName := strings.TrimSpace(o.Name)
	if signerName == "" {
		signerName = s.Cert.Subject.CommonName
	}
	now := time.Now()

	// Signature value with placeholders.
	sigNr := alloc()
	var sig bytes.Buffer
	sig.WriteString("<< /Type /Sig /Filter /Adobe.PPKLite /SubFilter /adbe.pkcs7.detached ")
	sig.WriteString("/ByteRange [0 0000000000 0000000000 0000000000] ")
	sig.WriteString("/Contents <" + strings.Repeat("0", 2*sigSize) + "> ")
	fmt.Fprintf(&sig, "/M %s /Name %s", pdfText(pdfDate(now)), pdfText(signerName))
	if o.Reason != "" {
		fmt.Fprintf(&sig, " /Reason %s", pdfText(o.Reason))
	}
	if o.Location != "" {
		fmt.Fprintf(&sig, " /Location %s", pdfText(o.Location))
	}
	if o.Contact != "" {
		fmt.Fprintf(&sig, " /ContactInfo %s", pdfText(o.Contact))
	}
	sig.WriteString(" >>")
	objs = append(objs, pdfObj{nr: sigNr, body: sig.String()})

	// Widget + field.
	widgetNr := alloc()
	rect := "[0 0 0 0]"
	ap := ""
	if o.Visible {
		dw, dh := geom.DisplaySize()
		lines := []string{i18n.L("Ditandatangani secara digital oleh", "Digitally signed by"), signerName, i18n.L("Tanggal: ", "Date: ") + now.Format("2006-01-02 15:04:05 -07:00")}
		if o.Reason != "" {
			lines = append(lines, i18n.L("Alasan: ", "Reason: ")+o.Reason)
		}
		if o.Location != "" {
			lines = append(lines, i18n.L("Lokasi: ", "Location: ")+o.Location)
		}
		const fs = 7.0
		w := 0.0
		for _, l := range lines {
			w = max(w, TextWidth(l, fs, false))
		}
		w += 12
		h := float64(len(lines))*fs*1.25 + 8
		margin := 28.0
		x, y := margin, margin // display space, origin bottom-left
		switch o.Position {
		case "br":
			x = dw - w - margin
		case "tl":
			y = dh - h - margin
		case "tr":
			x, y = dw-w-margin, dh-h-margin
		}
		ux1, uy1 := displayToUser(geom, x, y)
		ux2, uy2 := displayToUser(geom, x+w, y+h)
		rect = fmt.Sprintf("[%s %s %s %s]", num(min(ux1, ux2)), num(min(uy1, uy2)), num(max(ux1, ux2)), num(max(uy1, uy2)))
		fontNr := alloc()
		objs = append(objs, pdfObj{nr: fontNr, body: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"})
		var content bytes.Buffer
		fmt.Fprintf(&content, "q 0.96 0.97 1 rg 0.2 0.35 0.75 RG 0.8 w 0.4 0.4 %s %s re B Q\n", num(w-0.8), num(h-0.8))
		content.WriteString("BT 0.1 0.15 0.35 rg\n")
		for i, l := range lines {
			fmt.Fprintf(&content, "/F1 %s Tf 1 0 0 1 6 %s Tm %s Tj\n", num(fs), num(h-4-fs-float64(i)*fs*1.25), pdfString(winAnsi(l)))
		}
		content.WriteString("ET\n")
		apNr := alloc()
		matrix := map[int]string{90: "[0 1 -1 0 0 0]", 180: "[-1 0 0 -1 0 0]", 270: "[0 -1 1 0 0 0]"}[geom.Rotate]
		if matrix == "" {
			matrix = "[1 0 0 1 0 0]"
		}
		objs = append(objs, pdfObj{nr: apNr, body: fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 %s %s] /Matrix %s /Resources << /Font << /F1 %d 0 R >> >> /Length %d >>\nstream\n%s\nendstream",
			num(w), num(h), matrix, fontNr, content.Len(), content.String())})
		ap = fmt.Sprintf(" /AP << /N %d 0 R >>", apNr)
	}
	fieldName := fmt.Sprintf("KmbSignature%d", now.Unix())
	objs = append(objs, pdfObj{nr: widgetNr, body: fmt.Sprintf("<< /Type /Annot /Subtype /Widget /FT /Sig /T %s /V %d 0 R /F 132 /P %s /Rect %s%s >>",
		pdfText(fieldName), sigNr, pageRef.PDFString(), rect, ap)})

	// AcroForm with the new field.
	form := types.NewDict()
	if o, ok := catalog.Find("AcroForm"); ok && o != nil {
		if d, err := ctx.DereferenceDict(o); err == nil && d != nil {
			for k, v := range d {
				form[k] = v
			}
		}
	}
	fields := types.Array{}
	if o, ok := form.Find("Fields"); ok && o != nil {
		if arr, err := ctx.DereferenceArray(o); err == nil {
			fields = append(fields, arr...)
		}
	}
	fields = append(fields, *types.NewIndirectRef(widgetNr, 0))
	form["Fields"] = fields
	form["SigFlags"] = types.Integer(3)
	formNr := alloc()
	objs = append(objs, pdfObj{nr: formNr, body: form.PDFString()})

	newCatalog := types.NewDict()
	for k, v := range catalog {
		newCatalog[k] = v
	}
	newCatalog["AcroForm"] = *types.NewIndirectRef(formNr, 0)
	objs = append(objs, pdfObj{nr: root.ObjectNumber.Value(), gen: root.GenerationNumber.Value(), body: newCatalog.PDFString()})

	// Page annotations.
	widgetRef := *types.NewIndirectRef(widgetNr, 0)
	if a, ok := pageDict.Find("Annots"); ok && a != nil {
		if ir, isRef := a.(types.IndirectRef); isRef {
			arr, err := ctx.DereferenceArray(ir)
			if err != nil {
				return err
			}
			arr = append(append(types.Array{}, arr...), widgetRef)
			objs = append(objs, pdfObj{nr: ir.ObjectNumber.Value(), gen: ir.GenerationNumber.Value(), body: arr.PDFString()})
		} else {
			arr, _ := a.(types.Array)
			newPage := copyDict(pageDict)
			newPage["Annots"] = append(append(types.Array{}, arr...), widgetRef)
			objs = append(objs, pdfObj{nr: pageRef.ObjectNumber.Value(), gen: pageRef.GenerationNumber.Value(), body: newPage.PDFString()})
		}
	} else {
		newPage := copyDict(pageDict)
		newPage["Annots"] = types.Array{widgetRef}
		objs = append(objs, pdfObj{nr: pageRef.ObjectNumber.Value(), gen: pageRef.GenerationNumber.Value(), body: newPage.PDFString()})
	}

	// Incremental update.
	var buf bytes.Buffer
	buf.Write(data)
	if !bytes.HasSuffix(data, []byte("\n")) {
		buf.WriteByte('\n')
	}
	offsets := map[int]int{}
	gens := map[int]int{}
	for _, ob := range objs {
		offsets[ob.nr] = buf.Len()
		gens[ob.nr] = ob.gen
		fmt.Fprintf(&buf, "%d %d obj\n%s\nendobj\n", ob.nr, ob.gen, ob.body)
	}
	xref := buf.Len()
	buf.WriteString("xref\n")
	nrs := make([]int, 0, len(offsets))
	for nr := range offsets {
		nrs = append(nrs, nr)
	}
	sort.Ints(nrs)
	for i := 0; i < len(nrs); {
		j := i
		for j+1 < len(nrs) && nrs[j+1] == nrs[j]+1 {
			j++
		}
		fmt.Fprintf(&buf, "%d %d\n", nrs[i], j-i+1)
		for k := i; k <= j; k++ {
			fmt.Fprintf(&buf, "%010d %05d n\r\n", offsets[nrs[k]], gens[nrs[k]])
		}
		i = j + 1
	}
	trailer := fmt.Sprintf("<< /Size %d /Root %s /Prev %d", next, root.PDFString(), prev)
	if info := ctx.XRefTable.Info; info != nil {
		trailer += " /Info " + info.PDFString()
	}
	if id := ctx.XRefTable.ID; len(id) == 2 {
		trailer += " /ID [" + id[0].PDFString() + " " + id[1].PDFString() + "]"
	}
	trailer += " >>"
	fmt.Fprintf(&buf, "trailer\n%s\nstartxref\n%d\n%%%%EOF\n", trailer, xref)
	file := buf.Bytes()

	// Fill the byte range and sign.
	sigAt := offsets[sigNr]
	contentsAt := bytes.Index(file[sigAt:], []byte("/Contents <")) + sigAt + len("/Contents ")
	contentsEnd := contentsAt + 2*sigSize + 2
	br := fmt.Sprintf("[0 %d %d %d]", contentsAt, contentsEnd, len(file)-contentsEnd)
	brAt := bytes.Index(file[sigAt:], []byte("/ByteRange [")) + sigAt + len("/ByteRange ")
	placeholder := "[0 0000000000 0000000000 0000000000]"
	if len(br) > len(placeholder) {
		return errors.New("byte range too large")
	}
	copy(file[brAt:], br+strings.Repeat(" ", len(placeholder)-len(br)))

	h := sha256.New()
	h.Write(file[:contentsAt])
	h.Write(file[contentsEnd:])
	blob, err := s.pkcs7(h.Sum(nil), now)
	if err != nil {
		return err
	}
	if len(blob) > sigSize {
		return errors.New(i18n.L("rantai sertifikat terlalu besar", "the certificate chain is too large"))
	}
	hx := strings.ToUpper(hex.EncodeToString(blob))
	copy(file[contentsAt+1:], hx)
	return os.WriteFile(out, file, 0o644)
}

type pdfObj struct {
	nr, gen int
	body    string
}

func copyDict(d types.Dict) types.Dict {
	out := types.NewDict()
	for k, v := range d {
		out[k] = v
	}
	return out
}

// displayToUser maps a point in display space (origin bottom-left, y up) to user space.
func displayToUser(g Geom, x, y float64) (float64, float64) {
	switch g.Rotate {
	case 90:
		return g.X1 - y, g.Y0 + x
	case 180:
		return g.X1 - x, g.Y1 - y
	case 270:
		return g.X0 + y, g.Y1 - x
	}
	return g.X0 + x, g.Y0 + y
}

var reStartXref = regexp.MustCompile(`startxref\s+(\d+)\s+%%EOF\s*$`)

func lastStartXref(data []byte) (int, error) {
	tail := data
	if len(tail) > 2048 {
		tail = tail[len(tail)-2048:]
	}
	m := reStartXref.FindSubmatch(tail)
	if m == nil {
		return 0, errors.New("startxref not found")
	}
	return strconv.Atoi(string(m[1]))
}

// pdfText encodes a text string: PDFDocEncoding-compatible ASCII as is, else UTF-16BE.
func pdfText(s string) string {
	ascii := true
	for _, r := range s {
		if r > 126 || r < 32 {
			ascii = false
			break
		}
	}
	if ascii {
		return pdfString([]byte(s))
	}
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, r := range s {
		b.WriteString(utf16Hex(r))
	}
	b.WriteString(">")
	return b.String()
}

func pdfDate(t time.Time) string {
	_, off := t.Zone()
	sign := "+"
	if off < 0 {
		sign, off = "-", -off
	}
	return fmt.Sprintf("D:%s%s%02d'%02d'", t.Format("20060102150405"), sign, off/3600, off%3600/60)
}

// ---- CMS / PKCS#7 SignedData ---------------------------------------------------------

var (
	oidData          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidContentType   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSigningTime   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
	oidSigningCertV2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}
	oidSHA256        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSA           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidECDSASHA256   = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
)

func der(tag byte, parts ...[]byte) []byte {
	var body []byte
	for _, p := range parts {
		body = append(body, p...)
	}
	n := len(body)
	out := []byte{tag}
	switch {
	case n < 0x80:
		out = append(out, byte(n))
	case n < 0x100:
		out = append(out, 0x81, byte(n))
	case n < 0x10000:
		out = append(out, 0x82, byte(n>>8), byte(n))
	default:
		out = append(out, 0x83, byte(n>>16), byte(n>>8), byte(n))
	}
	return append(out, body...)
}

func seq(parts ...[]byte) []byte { return der(0x30, parts...) }

// set encodes a SET OF with its elements in DER order.
func set(parts ...[]byte) []byte {
	sorted := append([][]byte(nil), parts...)
	sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i], sorted[j]) < 0 })
	return der(0x31, sorted...)
}

func mustMarshal(v any) []byte {
	b, err := asn1.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func (s *Signer) pkcs7(digest []byte, at time.Time) ([]byte, error) {
	certHash := sha256.Sum256(s.Cert.Raw)
	attrs := [][]byte{
		seq(mustMarshal(oidContentType), set(mustMarshal(oidData))),
		seq(mustMarshal(oidSigningTime), set(mustMarshal(at.UTC()))),
		seq(mustMarshal(oidMessageDigest), set(mustMarshal(digest))),
		seq(mustMarshal(oidSigningCertV2), set(seq(seq(seq(mustMarshal(certHash[:])))))),
	}
	signedAttrs := set(attrs...)
	h := sha256.Sum256(signedAttrs)
	signature, err := s.Key.Sign(rand.Reader, h[:], crypto.SHA256)
	if err != nil {
		return nil, err
	}
	sigAlg := seq(mustMarshal(oidRSA), []byte{0x05, 0x00})
	if _, ok := s.Key.(*ecdsa.PrivateKey); ok {
		sigAlg = seq(mustMarshal(oidECDSASHA256))
	}
	digestAlg := seq(mustMarshal(oidSHA256))
	serial := mustMarshal(s.Cert.SerialNumber)
	signerInfo := seq(
		mustMarshal(1),
		seq(s.Cert.RawIssuer, serial),
		digestAlg,
		der(0xA0, signedAttrs[headerLen(signedAttrs):]), // [0] IMPLICIT
		sigAlg,
		der(0x04, signature),
	)
	certs := [][]byte{s.Cert.Raw}
	for _, c := range s.Chain {
		certs = append(certs, c.Raw)
	}
	signedData := seq(
		mustMarshal(1),
		set(digestAlg),
		seq(mustMarshal(oidData)),
		der(0xA0, certs...),
		set(signerInfo),
	)
	return seq(mustMarshal(oidSignedData), der(0xA0, signedData)), nil
}

// headerLen is the length of a DER tag+length header.
func headerLen(b []byte) int {
	if len(b) < 2 || b[1] < 0x80 {
		return 2
	}
	return 2 + int(b[1]&0x7F)
}
