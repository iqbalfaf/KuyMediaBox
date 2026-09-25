package imageconv

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// writeNoisyJPEG writes a photo-like JPEG (noise compresses badly, like real photos) with EXIF.
func writeNoisyJPEG(t *testing.T, path string, w, h int, orientation uint16) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(1))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(rng.Intn(256)), uint8(x), uint8(y), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	// Minimal EXIF: IFD0 with Orientation and Make.
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08")
	tiff = binary.BigEndian.AppendUint16(tiff, 2)
	tiff = append(tiff, 0x01, 0x12, 0x00, 0x03, 0, 0, 0, 1)
	tiff = binary.BigEndian.AppendUint16(tiff, orientation)
	tiff = append(tiff, 0, 0)
	tiff = append(tiff, 0x01, 0x0F, 0x00, 0x02, 0, 0, 0, 4, 'K', 'M', 'B', 0)
	tiff = append(tiff, 0, 0, 0, 0)
	data := embedExif(buf.Bytes(), "jpg", append([]byte("Exif\x00\x00"), tiff...))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTransformSizes(t *testing.T) {
	cases := []struct {
		o    Options
		want Size
	}{
		{Options{Rotate: 90}, Size{200, 300}},
		{Options{Crop: "1:1"}, Size{200, 200}},
		{Options{Crop: "16:9"}, Size{300, 169}},
		{Options{Rotate: 270, Crop: "4:5"}, Size{200, 250}},
		{Options{Format: "ico", IcoSizes: []int{16, 256, 48}}, Size{256, 256}},
	}
	for _, c := range cases {
		if got := TargetSize(300, 200, c.o); got != c.want {
			t.Errorf("%+v: got %v want %v", c.o, got, c.want)
		}
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.png")
	writeTestPNG(t, src, 300, 200, false)
	out := filepath.Join(dir, "a.png.out.png")
	if err := Convert(context.Background(), src, out, Options{Format: "png", Rotate: 90, Crop: "1:1", FlipH: true}, "", func(float64) {}); err != nil {
		t.Fatal(err)
	}
	size, _, err := Config(out)
	if err != nil || size.W != 200 || size.H != 200 {
		t.Fatalf("size %v err %v", size, err)
	}
}

func TestTargetKB(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	writeNoisyJPEG(t, src, 800, 600, 1)
	for _, format := range []string{"jpg", "webp", "png"} {
		out := filepath.Join(dir, "out."+format)
		if err := Convert(context.Background(), src, out, Options{Format: format, Quality: 95, TargetKB: 60}, "", func(float64) {}); err != nil {
			t.Fatal(format, err)
		}
		st, _ := os.Stat(out)
		if st.Size() > 60*1024 {
			t.Errorf("%s: %d bytes, want ≤ 60 KB", format, st.Size())
		}
	}
}

func TestICOSizes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "logo.png")
	writeTestPNG(t, src, 300, 200, true)
	out := filepath.Join(dir, "logo.ico")
	if err := Convert(context.Background(), src, out, Options{Format: "ico", IcoSizes: []int{16, 32, 48, 256}}, "", func(float64) {}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if n := binary.LittleEndian.Uint16(data[4:]); n != 4 {
		t.Fatalf("%d images, want 4", n)
	}
	img, err := decodeICO(data)
	if err != nil || img.Bounds().Dx() != 256 {
		t.Fatalf("decode: %v %v", err, img)
	}
}

func TestExifKeepAndWatermark(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cam.jpg")
	writeNoisyJPEG(t, src, 400, 300, 6)
	keep := filepath.Join(dir, "keep.jpg")
	if err := Convert(context.Background(), src, keep, Options{Format: "jpg", KeepMetadata: true, AutoRotate: true}, "", func(float64) {}); err != nil {
		t.Fatal(err)
	}
	exif := readExif(keep)
	if exif == nil || !bytes.Contains(exif, []byte("KMB")) {
		t.Fatal("EXIF not kept")
	}
	if !bytes.Contains(exif, []byte{0x01, 0x12, 0x00, 0x03, 0, 0, 0, 1, 0, 1}) {
		t.Fatal("orientation not reset to 1")
	}
	strip := filepath.Join(dir, "strip.jpg")
	wm := Watermark{Enabled: true, Type: "text", Text: "© KuyMediaBox Привет", Size: 10, Opacity: 1, Color: "#ff0000", Position: "c"}
	if err := Convert(context.Background(), src, strip, Options{Format: "png", Watermark: wm}, "", func(float64) {}); err != nil {
		t.Fatal(err)
	}
	if readExif(strip) != nil {
		t.Fatal("EXIF should be removed")
	}
	f, _ := os.Open(strip)
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	red := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r > 0xE000 && g < 0x3000 && bl < 0x3000 {
				red++
			}
		}
	}
	if red < 100 {
		t.Fatalf("watermark not drawn (%d red pixels)", red)
	}
}
