package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/bgremove"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/tools"
)

// AI picture tools for the Image page: Real-ESRGAN upscaling, background removal (ONNX
// Runtime) and PNG compression (pngquant, oxipng). imageconv calls them through its hooks.

var (
	upscaleMu sync.Mutex // one GPU job at a time
	removerMu sync.Mutex
	remover   *bgremove.Remover
	removerOf string // dll the remover was made for
)

func (a *App) installImageHooks() {
	imageconv.Hooks.Upscale = a.aiUpscale
	imageconv.Hooks.RemoveBG = a.aiRemoveBG
	imageconv.Hooks.OptimizePNG = a.optimizePNG
}

// maxUpscalePixels keeps AI upscaling within memory: the size, in pixels, of the 4× picture
// Real-ESRGAN always makes (2× and 3× are scaled down from it afterwards).
const maxUpscalePixels = 120_000_000

func (a *App) aiUpscale(ctx context.Context, img image.Image, scale int, model string) (image.Image, error) {
	exe := a.tools.Path(tools.RealESRGAN)
	if exe == "" {
		return nil, errors.New(i18n.L("Real-ESRGAN belum terpasang (Pengaturan › Tools pendukung)", "Real-ESRGAN isn't installed (Settings › Supporting tools)"))
	}
	b := img.Bounds()
	if b.Dx()*b.Dy()*16 > maxUpscalePixels {
		mp := float64(b.Dx()*b.Dy()) / 1e6
		return nil, fmt.Errorf(i18n.L("Gambar terlalu besar untuk Perbesar AI (%.1f megapiksel, maks. %.1f). Perkecil dulu.", "The picture is too big for AI upscaling (%.1f megapixels, max %.1f). Make it smaller first."), mp, float64(maxUpscalePixels)/16/1e6)
	}
	work, err := os.MkdirTemp(appdir.TempDir(), "esr-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	in, out := filepath.Join(work, "in.png"), filepath.Join(work, "out.png")
	if err := savePNG(in, img); err != nil {
		return nil, err
	}
	name := "realesrgan-x4plus"
	if model == "anime" {
		name = "realesrgan-x4plus-anime"
	}
	upscaleMu.Lock()
	_, err = proc.Output(ctx, exe, "-i", in, "-o", out, "-n", name, "-s", "4", "-m", filepath.Join(filepath.Dir(exe), "models"), "-f", "png")
	upscaleMu.Unlock()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "vulkan") || strings.Contains(msg, "invalid gpu") {
			return nil, errors.New(i18n.L("Perbesar AI butuh kartu grafis yang mendukung Vulkan (driver terbaru).", "AI upscaling needs a graphics card with Vulkan support (latest driver)."))
		}
		return nil, fmt.Errorf(i18n.L("Real-ESRGAN gagal: %s", "Real-ESRGAN failed: %s"), firstLineOf(msg))
	}
	res, err := imaging.Open(out)
	if err != nil {
		return nil, errors.New(i18n.L("Real-ESRGAN tidak menghasilkan gambar (kartu grafis tidak didukung?)", "Real-ESRGAN produced no picture (graphics card not supported?)"))
	}
	if scale != 4 {
		res = imaging.Resize(res, b.Dx()*scale, b.Dy()*scale, imaging.Lanczos)
	}
	return res, nil
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (a *App) bgRemover() (*bgremove.Remover, error) {
	dll := a.tools.Path(tools.OnnxRuntime)
	if dll == "" {
		return nil, errors.New(i18n.L("ONNX Runtime belum terpasang (Pengaturan › Tools pendukung)", "ONNX Runtime isn't installed (Settings › Supporting tools)"))
	}
	removerMu.Lock()
	defer removerMu.Unlock()
	if remover == nil || removerOf != dll {
		if remover != nil {
			remover.Close()
		}
		remover, removerOf = bgremove.New(dll, tools.ModelDir("bgremove")), dll
	}
	return remover, nil
}

// resetRemover forgets loaded models (before one is deleted).
func (a *App) resetRemover() {
	removerMu.Lock()
	defer removerMu.Unlock()
	if remover != nil {
		remover.Close()
		remover = nil
	}
}

func (a *App) aiRemoveBG(ctx context.Context, img image.Image, model string, maskOnly bool) (image.Image, error) {
	m, ok := bgremove.Find(model)
	if !ok {
		return nil, errors.New(i18n.L("model tidak dikenal", "unknown model"))
	}
	if def, ok := findModel("bgremove", model); !ok || !modelInstalled("bgremove", def) {
		return nil, errors.New(i18n.L("Model hapus latar belum diunduh (lihat panel Hapus latar)", "The background model hasn't been downloaded (see the Remove background panel)"))
	}
	r, err := a.bgRemover()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if maskOnly {
		mask, err := r.Mask(img, m)
		if err != nil {
			return nil, err
		}
		return bgremove.MaskImage(mask), nil
	}
	return r.Cutout(img, m)
}

func (a *App) optimizePNG(ctx context.Context, path, mode string) error {
	quant, oxi := a.tools.Path(tools.Pngquant), a.tools.Path(tools.Oxipng)
	if mode == "small" {
		if quant == "" {
			return errors.New(i18n.L("pngquant belum terpasang (Pengaturan › Tools pendukung)", "pngquant isn't installed (Settings › Supporting tools)"))
		}
		// Exit 98/99 = the result would be bigger or too ugly: keep the normal PNG.
		_, _ = proc.Output(ctx, quant, "--quality=60-90", "--speed", "3", "--strip", "--skip-if-larger", "--force", "--output", path, "--", path)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	} else if oxi == "" {
		return errors.New(i18n.L("oxipng belum terpasang (Pengaturan › Tools pendukung)", "oxipng isn't installed (Settings › Supporting tools)"))
	}
	if oxi != "" {
		if _, err := proc.Output(ctx, oxi, "-o", "3", "--strip", "safe", "-q", path); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf(i18n.L("oxipng gagal: %s", "oxipng failed: %s"), firstLineOf(err.Error()))
		}
	}
	return nil
}
