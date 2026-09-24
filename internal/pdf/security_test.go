package pdf

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestProtectUnlock(t *testing.T) {
	dir := t.TempDir()
	src := makePDF(t, dir, "a.pdf", 2)
	bg := context.Background()

	locked := filepath.Join(dir, "locked.pdf")
	if err := Protect(Input{Path: src}, ProtectOptions{Password: "rahasia", AllowPrint: true}, locked, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(bg, locked, ""); !errors.Is(err, ErrPassword) {
		t.Fatalf("open without password: %v", err)
	}
	if d, err := Open(bg, locked, "rahasia"); err != nil {
		t.Fatal(err)
	} else {
		d.Close()
	}
	if info, _ := Inspect(bg, locked); !info.Locked {
		t.Fatal("inspect must report locked")
	}
	// Other tools work with the password and write an unprotected result.
	rot := filepath.Join(dir, "rot.pdf")
	if err := Rotate(Input{Path: locked, Password: "rahasia"}, 90, "", rot); err != nil {
		t.Fatal(err)
	}
	if info, _ := Inspect(bg, rot); info.Encrypted {
		t.Fatal("rotated copy must not be encrypted")
	}
	if err := Unlock(Input{Path: locked, Password: "salah"}, filepath.Join(dir, "x.pdf")); err == nil {
		t.Fatal("wrong password must fail")
	}
	open := filepath.Join(dir, "open.pdf")
	if err := Unlock(Input{Path: locked, Password: "rahasia"}, open); err != nil {
		t.Fatal(err)
	}
	if info, _ := Inspect(bg, open); info.Encrypted || info.Pages != 2 {
		t.Fatalf("unlocked: %+v", info)
	}
	if err := Unlock(Input{Path: open}, filepath.Join(dir, "y.pdf")); err == nil {
		t.Fatal("unlocking an open file must say it isn't locked")
	}

	// Restrictions only (no open password): tools work without a password, unlock removes them.
	ownerOnly := filepath.Join(dir, "owneronly.pdf")
	c := model.NewAESConfiguration("", "owner", 256)
	c.Permissions = model.PermissionsNone
	if err := api.EncryptFile(src, ownerOnly, c); err != nil {
		t.Fatal(err)
	}
	if info, _ := Inspect(bg, ownerOnly); info.Locked || !info.Encrypted {
		t.Fatalf("owner-only: %+v", info)
	}
	if err := Rotate(Input{Path: ownerOnly}, 90, "", filepath.Join(dir, "r2.pdf")); err != nil {
		t.Fatalf("rotate restricted: %v", err)
	}
	if err := Unlock(Input{Path: ownerOnly}, filepath.Join(dir, "u2.pdf")); err != nil {
		t.Fatalf("unlock restricted: %v", err)
	}
}
