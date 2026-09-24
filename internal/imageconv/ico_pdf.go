package imageconv

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/disintegration/imaging"
	"golang.org/x/image/bmp"
)

// encodeICO writes a single-image ICO with PNG payload (supported since Windows Vista).
func encodeICO(w io.Writer, img image.Image) error {
	b := img.Bounds()
	if b.Dx() > 256 || b.Dy() > 256 {
		img = imaging.Fit(img, 256, 256, imaging.Lanczos)
		b = img.Bounds()
	}
	var payload bytes.Buffer
	if err := png.Encode(&payload, img); err != nil {
		return err
	}
	dim := func(v int) byte {
		if v >= 256 {
			return 0 // 0 means 256 in the ICO directory
		}
		return byte(v)
	}
	header := []byte{0, 0, 1, 0, 1, 0}
	entry := make([]byte, 16)
	entry[0] = dim(b.Dx())
	entry[1] = dim(b.Dy())
	binary.LittleEndian.PutUint16(entry[4:], 1)  // planes
	binary.LittleEndian.PutUint16(entry[6:], 32) // bpp
	binary.LittleEndian.PutUint32(entry[8:], uint32(payload.Len()))
	binary.LittleEndian.PutUint32(entry[12:], 6+16)
	for _, part := range [][]byte{header, entry, payload.Bytes()} {
		if _, err := w.Write(part); err != nil {
			return err
		}
	}
	return nil
}

// decodeICO picks the largest image in an ICO file (PNG or BMP payload).
func decodeICO(data []byte) (image.Image, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, errors.New("bukan file ICO")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	bestSize, bestOff, bestLen := -1, 0, 0
	for i := 0; i < count; i++ {
		e := 6 + i*16
		if e+16 > len(data) {
			break
		}
		wd, ht := int(data[e]), int(data[e+1])
		if wd == 0 {
			wd = 256
		}
		if ht == 0 {
			ht = 256
		}
		size := int(binary.LittleEndian.Uint32(data[e+8:]))
		off := int(binary.LittleEndian.Uint32(data[e+12:]))
		if off < 0 || size <= 0 || off+size > len(data) {
			continue
		}
		if wd*ht > bestSize {
			bestSize, bestOff, bestLen = wd*ht, off, size
		}
	}
	if bestSize < 0 {
		return nil, errors.New("ICO kosong")
	}
	payload := data[bestOff : bestOff+bestLen]
	if bytes.HasPrefix(payload, []byte("\x89PNG")) {
		return png.Decode(bytes.NewReader(payload))
	}
	// BMP payload without file header and with doubled height (XOR + AND masks).
	if len(payload) < 40 {
		return nil, errors.New("ICO rusak")
	}
	hdrSize := binary.LittleEndian.Uint32(payload[0:])
	fixed := append([]byte(nil), payload...)
	h := int32(binary.LittleEndian.Uint32(fixed[8:]))
	binary.LittleEndian.PutUint32(fixed[8:], uint32(h/2))
	bpp := binary.LittleEndian.Uint16(fixed[14:])
	colors := binary.LittleEndian.Uint32(fixed[32:])
	if colors == 0 && bpp <= 8 {
		colors = 1 << bpp
	}
	fileHdr := make([]byte, 14)
	fileHdr[0], fileHdr[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(fileHdr[2:], uint32(14+len(fixed)))
	binary.LittleEndian.PutUint32(fileHdr[10:], 14+hdrSize+colors*4)
	return bmp.Decode(bytes.NewReader(append(fileHdr, fixed...)))
}

// encodePDF writes a one-page PDF containing the image as JPEG, page size = image at 96 dpi.
func encodePDF(w io.Writer, img image.Image, quality int) error {
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, &jpeg.Options{Quality: quality}); err != nil {
		return err
	}
	b := img.Bounds()
	pw := float64(b.Dx()) * 72 / 96
	ph := float64(b.Dy()) * 72 / 96
	colorSpace := "/DeviceRGB"
	if _, gray := img.(*image.Gray); gray {
		colorSpace = "/DeviceGray"
	}
	content := fmt.Sprintf("q %.2f 0 0 %.2f 0 0 cm /Im0 Do Q", pw, ph)

	var buf bytes.Buffer
	offsets := make([]int, 6)
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	obj := func(n int, body string) {
		offsets[n] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	obj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	obj(3, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>", pw, ph))
	offsets[4] = buf.Len()
	fmt.Fprintf(&buf, "4 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
		b.Dx(), b.Dy(), colorSpace, jpg.Len())
	buf.Write(jpg.Bytes())
	buf.WriteString("\nendstream\nendobj\n")
	offsets[5] = buf.Len()
	fmt.Fprintf(&buf, "5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content)
	xref := buf.Len()
	buf.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	_, err := w.Write(buf.Bytes())
	return err
}
