//go:build integration

package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kuymediabox/internal/config"
)

// TestInstallExtraTools downloads the new optional tools into a temporary data folder and
// checks that each one is found and runs.
func TestInstallExtraTools(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	m := New(config.Load(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	for _, id := range []string{Oxipng, Pngquant, Whisper, RealESRGAN, OnnxRuntime} {
		start := time.Now()
		if err := m.Install(ctx, id); err != nil {
			t.Fatalf("install %s: %v", id, err)
		}
		st := m.statuses[id]
		t.Logf("%s %s at %s (%s)", id, st.Version, st.Path, time.Since(start).Round(time.Second))
		if !st.Found || st.Version == "" || st.Version == "?" {
			t.Errorf("%s: %+v", id, *st)
		}
	}
	for _, f := range []string{"whisper-cli.exe", "whisper.dll", "ggml.dll"} {
		if _, err := os.Stat(filepath.Join(ToolDir(Whisper), f)); err != nil {
			t.Errorf("whisper: %s missing", f)
		}
	}
	if _, err := os.Stat(filepath.Join(ToolDir(RealESRGAN), "models", "realesrgan-x4plus.param")); err != nil {
		t.Error("Real-ESRGAN models missing")
	}
	// Reinstalling keeps downloaded models.
	models := ModelDir("whisper")
	_ = os.MkdirAll(models, 0o755)
	_ = os.WriteFile(filepath.Join(models, "keep.bin"), []byte("x"), 0o644)
	if err := m.Install(ctx, Whisper); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(models, "keep.bin")); err != nil {
		t.Error("reinstalling whisper lost the models")
	}
}
