package pdf

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"kuymediabox/internal/i18n"
)

// ProtectOptions control password protection.
type ProtectOptions struct {
	Password      string `json:"password"`      // needed to open the file
	OwnerPassword string `json:"ownerPassword"` // needed to change permissions ("" = random when restricted)
	AllowPrint    bool   `json:"allowPrint"`
	AllowCopy     bool   `json:"allowCopy"`
	AllowEdit     bool   `json:"allowEdit"`
	AES128        bool   `json:"aes128"` // older, more compatible encryption
}

// Protect encrypts a PDF with AES (256-bit unless AES128).
func Protect(in Input, o ProtectOptions, out, tmpDir string) error {
	if o.Password == "" {
		return errors.New(i18n.L("isi password", "enter a password"))
	}
	src := in.Path
	// Start from an unencrypted copy when the source is protected already.
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	if ctx.Encrypt != nil {
		plain := filepath.Join(tmpDir, "plain-"+randomHex(6)+".pdf")
		if err := writeCtx(ctx, plain); err != nil {
			return err
		}
		defer os.Remove(plain)
		src = plain
	}
	perm := model.PermissionsNone
	if o.AllowPrint {
		perm |= model.PermissionPrintRev2 | model.PermissionPrintRev3
	}
	if o.AllowCopy {
		perm |= model.PermissionExtract | model.PermissionExtractRev3
	}
	if o.AllowEdit {
		perm |= model.PermissionModify | model.PermissionModAnnFillForm | model.PermissionFillRev3 | model.PermissionAssembleRev3
	}
	owner := o.OwnerPassword
	if owner == "" {
		if perm == model.PermissionsAll || (o.AllowPrint && o.AllowCopy && o.AllowEdit) {
			owner = o.Password
		} else {
			// A secret owner password keeps the restrictions in force.
			owner = randomHex(16)
		}
	}
	c := model.NewAESConfiguration(o.Password, owner, 256)
	if o.AES128 {
		c = model.NewAESConfiguration(o.Password, owner, 128)
	}
	c.ValidationMode = model.ValidationRelaxed
	c.Permissions = perm
	if err := api.EncryptFile(src, out, c); err != nil {
		_ = os.Remove(out)
		return friendly(err)
	}
	return nil
}

// Unlock removes the password and restrictions of a PDF.
func Unlock(in Input, out string) error {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		if in.Password == "" {
			return errors.New(i18n.L("PDF ini butuh password untuk dibuka — isi passwordnya", "this PDF needs a password to open — enter it"))
		}
		return errors.New(i18n.L("password salah", "wrong password"))
	}
	if ctx.Encrypt == nil {
		return errors.New(i18n.L("PDF ini tidak dikunci", "this PDF isn't locked"))
	}
	return writeCtx(ctx, out)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
