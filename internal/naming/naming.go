// Package naming decides where converted files go and what they are called, without ever
// overwriting the source file or colliding with another task running at the same time.
package naming

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"kuymediabox/internal/config"
)

// ErrExists is returned when the conflict policy is "skip" and the target already exists.
var ErrExists = errors.New("file hasil sudah ada")

// OutputSpec says where a module writes its results.
type OutputSpec struct {
	Mode string `json:"mode"` // subfolder | same | custom
	Dir  string `json:"dir"`  // used when Mode == custom
}

// SubfolderName is the folder created next to the source for Mode == subfolder.
const SubfolderName = "converted"

// Namer hands out unique output paths. Paths reserved by running tasks count as taken.
type Namer struct {
	mu       sync.Mutex
	reserved map[string]bool
}

// NewNamer returns an empty Namer.
func NewNamer() *Namer { return &Namer{reserved: map[string]bool{}} }

// Reserve picks the output path for input converted to ext (without dot).
// The returned release func must be called when the task ends.
func (n *Namer) Reserve(input string, spec OutputSpec, suffix, ext, conflict string) (string, func(), error) {
	dir, err := OutputDir(input, spec)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, fmt.Errorf("tidak bisa membuat folder hasil: %w", err)
	}
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	name := SanitizeFileName(base + suffix)
	target := filepath.Join(dir, name+"."+ext)

	n.mu.Lock()
	defer n.mu.Unlock()

	// Never write over the source file, whatever the policy says.
	sameAsSource := samePath(target, input)
	if sameAsSource || n.reserved[key(target)] {
		target = n.nextFree(dir, name, ext, input)
	} else if exists(target) {
		switch conflict {
		case config.ConflictSkip:
			return target, func() {}, ErrExists
		case config.ConflictOverwrite:
			// keep target
		default:
			target = n.nextFree(dir, name, ext, input)
		}
	}
	k := key(target)
	n.reserved[k] = true
	return target, func() {
		n.mu.Lock()
		delete(n.reserved, k)
		n.mu.Unlock()
	}, nil
}

func (n *Namer) nextFree(dir, name, ext, input string) string {
	for i := 1; i < 100000; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%s (%d).%s", name, i, ext))
		if !exists(p) && !n.reserved[key(p)] && !samePath(p, input) {
			return p
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s (%d).%s", name, os.Getpid(), ext))
}

// OutputDir resolves the output folder for one input file.
func OutputDir(input string, spec OutputSpec) (string, error) {
	src := filepath.Dir(input)
	switch spec.Mode {
	case config.OutputSame:
		return src, nil
	case config.OutputCustom:
		if strings.TrimSpace(spec.Dir) == "" {
			return "", errors.New("folder hasil belum dipilih")
		}
		return spec.Dir, nil
	default:
		return filepath.Join(src, SubfolderName), nil
	}
}

// TempPath returns a sibling temp path with the same extension (ffmpeg picks the muxer from it).
func TempPath(final string) string {
	ext := filepath.Ext(final)
	return strings.TrimSuffix(final, ext) + ".kmb-part" + ext
}

// Commit moves a finished temp file into place, replacing an existing target.
func Commit(tmp, final string) error {
	if _, err := os.Stat(tmp); err != nil {
		return fmt.Errorf("file hasil tidak ditemukan: %w", err)
	}
	if exists(final) {
		if err := os.Remove(final); err != nil {
			return fmt.Errorf("tidak bisa menimpa file lama: %w", err)
		}
	}
	return os.Rename(tmp, final)
}

// SanitizeFileName makes a string safe as a Windows file or folder name.
func SanitizeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 32:
			continue
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.TrimRight(out, ". ")
	if out == "" {
		out = "file"
	}
	upper := strings.ToUpper(out)
	for _, bad := range []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "LPT1", "LPT2", "LPT3"} {
		if upper == bad {
			out = "_" + out
		}
	}
	if r := []rune(out); len(r) > 150 {
		out = strings.TrimSpace(string(r[:150]))
	}
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func key(p string) string { return strings.ToLower(filepath.Clean(p)) }

func samePath(a, b string) bool {
	if key(a) == key(b) {
		return true
	}
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}
