// Package bgremove cuts the subject out of a picture with a segmentation model (U²-Net /
// IS-Net, the models rembg uses) run by ONNX Runtime.
package bgremove

import (
	"errors"
	"image"
	"math"
	"path/filepath"
	"sync"

	"github.com/disintegration/imaging"

	"kuymediabox/internal/onnx"
)

// Model is a segmentation model offered in the UI.
type Model struct {
	ID     string     `json:"id"`
	File   string     `json:"file"`
	SizeMB int        `json:"sizeMB"`
	side   int        // input size (square)
	mean   [3]float32 // per channel
	std    [3]float32
}

var imagenetMean, imagenetStd = [3]float32{0.485, 0.456, 0.406}, [3]float32{0.229, 0.224, 0.225}

// Models: fast and small, general purpose (best), and people.
var Models = []Model{
	{ID: "general", File: "isnet-general-use.onnx", SizeMB: 179, side: 1024, mean: [3]float32{0.5, 0.5, 0.5}, std: [3]float32{1, 1, 1}},
	{ID: "people", File: "u2net_human_seg.onnx", SizeMB: 176, side: 320, mean: imagenetMean, std: imagenetStd},
	{ID: "fast", File: "u2netp.onnx", SizeMB: 5, side: 320, mean: imagenetMean, std: imagenetStd},
}

// ModelURL is where a model file is downloaded from (rembg's model releases).
func ModelURL(m Model) string {
	return "https://github.com/danielgatis/rembg/releases/download/v0.0.0/" + m.File
}

// Find returns the model with an id.
func Find(id string) (Model, bool) {
	for _, m := range Models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// Remover keeps loaded models (loading IS-Net takes a second or two).
type Remover struct {
	mu       sync.Mutex
	dll      string
	dir      string
	sessions map[string]*onnx.Session
}

// New creates a remover using onnxruntime.dll and the model folder dir.
func New(dll, dir string) *Remover {
	return &Remover{dll: dll, dir: dir, sessions: map[string]*onnx.Session{}}
}

// Close frees the loaded models.
func (r *Remover) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		s.Close()
		delete(r.sessions, id)
	}
}

func (r *Remover) session(m Model) (*onnx.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.sessions[m.ID]; s != nil {
		return s, nil
	}
	rt, err := onnx.Load(r.dll)
	if err != nil {
		return nil, err
	}
	s, err := rt.Open(filepath.Join(r.dir, m.File), 0)
	if err != nil {
		return nil, err
	}
	r.sessions[m.ID] = s
	return s, nil
}

// Mask returns the subject mask (white = keep) at the size of img.
func (r *Remover) Mask(img image.Image, m Model) (*image.Gray, error) {
	s, err := r.session(m)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() < 2 || b.Dy() < 2 {
		return nil, errors.New("picture too small")
	}
	side := m.side
	small := imaging.Resize(img, side, side, imaging.Lanczos)
	// Like rembg: scale by the brightest value, then normalise per channel.
	maxV := uint8(1)
	for i := 0; i < len(small.Pix); i += 4 {
		maxV = max(maxV, small.Pix[i], small.Pix[i+1], small.Pix[i+2])
	}
	plane := side * side
	input := make([]float32, 3*plane)
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			p := small.Pix[y*small.Stride+x*4:]
			for c := 0; c < 3; c++ {
				v := float32(p[c]) / float32(maxV)
				input[c*plane+y*side+x] = (v - m.mean[c]) / m.std[c]
			}
		}
	}
	out, shape, err := s.Run(input, []int64{1, 3, int64(side), int64(side)})
	if err != nil {
		return nil, err
	}
	if len(shape) < 2 {
		return nil, errors.New("unexpected model output")
	}
	oh, ow := int(shape[len(shape)-2]), int(shape[len(shape)-1])
	if oh*ow > len(out) || oh <= 0 || ow <= 0 {
		return nil, errors.New("unexpected model output")
	}
	pred := out[:oh*ow]
	lo, hi := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, v := range pred {
		lo, hi = min(lo, v), max(hi, v)
	}
	span := hi - lo
	if span < 1e-6 {
		span = 1e-6
	}
	mask := image.NewGray(image.Rect(0, 0, ow, oh))
	for i, v := range pred {
		mask.Pix[i] = uint8(math.Round(float64((v - lo) / span * 255)))
	}
	full := imaging.Resize(mask, b.Dx(), b.Dy(), imaging.Lanczos)
	g := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for i := range g.Pix {
		g.Pix[i] = full.Pix[i*4] // NRGBA gray → one channel
	}
	return g, nil
}

// Cutout returns img with the background made transparent.
func (r *Remover) Cutout(img image.Image, m Model) (*image.NRGBA, error) {
	mask, err := r.Mask(img, m)
	if err != nil {
		return nil, err
	}
	src := imaging.Clone(img)
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			i := y*src.Stride + x*4
			a := uint16(src.Pix[i+3]) * uint16(mask.Pix[y*mask.Stride+x]) / 255
			src.Pix[i+3] = uint8(a)
		}
	}
	return src, nil
}

// MaskImage turns a mask into a black-and-white picture (for "mask only" output).
func MaskImage(g *image.Gray) *image.NRGBA {
	out := image.NewNRGBA(g.Bounds())
	for i, v := range g.Pix {
		out.Pix[i*4], out.Pix[i*4+1], out.Pix[i*4+2], out.Pix[i*4+3] = v, v, v, 255
	}
	return out
}
