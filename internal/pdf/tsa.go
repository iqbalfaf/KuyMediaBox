package pdf

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"kuymediabox/internal/i18n"
)

// RFC 3161 timestamps: a trusted time-stamping authority (TSA) signs the time at which the
// signature existed, so it stays valid after the certificate expires (PAdES-T).

var oidTimeStampToken = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}

// TSAServers are free public time-stamping services offered in the UI.
var TSAServers = []string{"http://timestamp.digicert.com", "http://timestamp.sectigo.com", "https://freetsa.org/tsr"}

var tsaClient = &http.Client{Timeout: 30 * time.Second}

type messageImprint struct {
	HashAlgorithm struct {
		Algorithm asn1.ObjectIdentifier
		Params    asn1.RawValue `asn1:"optional"`
	}
	HashedMessage []byte
}

type timeStampReq struct {
	Version        int
	MessageImprint messageImprint
	Nonce          *big.Int `asn1:"optional"`
	CertReq        bool     `asn1:"optional,default:false"`
}

type pkiStatusInfo struct {
	Status       int
	StatusString asn1.RawValue  `asn1:"optional"`
	FailInfo     asn1.BitString `asn1:"optional"`
}

type timeStampResp struct {
	Status pkiStatusInfo
	Token  asn1.RawValue `asn1:"optional"`
}

// timestampToken asks the TSA at url for a token over data (the signature value).
func timestampToken(ctx context.Context, url string, data []byte) ([]byte, error) {
	sum := sha256.Sum256(data)
	nonce, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 63))
	var req timeStampReq
	req.Version = 1
	req.MessageImprint.HashAlgorithm.Algorithm = oidSHA256
	req.MessageImprint.HashAlgorithm.Params = asn1.RawValue{Tag: 5} // NULL
	req.MessageImprint.HashedMessage = sum[:]
	req.Nonce = nonce
	req.CertReq = true
	body, err := asn1.Marshal(req)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/timestamp-query")
	resp, err := tsaClient.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf(i18n.L("server stempel waktu tidak bisa dihubungi: %w", "the time-stamp server can't be reached: %w"), err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(i18n.L("server stempel waktu menjawab %s", "the time-stamp server replied %s"), resp.Status)
	}
	var tr timeStampResp
	if _, err := asn1.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf(i18n.L("jawaban server stempel waktu tidak dikenali: %w", "unexpected time-stamp server reply: %w"), err)
	}
	if tr.Status.Status > 1 || len(tr.Token.FullBytes) == 0 {
		return nil, fmt.Errorf(i18n.L("server stempel waktu menolak permintaan (status %d)", "the time-stamp server refused the request (status %d)"), tr.Status.Status)
	}
	// The token must be for our data and our nonce.
	if !bytes.Contains(tr.Token.FullBytes, sum[:]) {
		return nil, errors.New(i18n.L("stempel waktu tidak cocok dengan tanda tangan", "the time stamp doesn't match the signature"))
	}
	if nb := nonce.Bytes(); len(nb) > 4 && !bytes.Contains(tr.Token.FullBytes, nb) {
		return nil, errors.New(i18n.L("stempel waktu tidak cocok dengan permintaan", "the time stamp doesn't match the request"))
	}
	return tr.Token.FullBytes, nil
}

// normalizeTSA returns a usable TSA address ("" = no time stamp).
func normalizeTSA(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "http://" + s
	}
	return s
}
