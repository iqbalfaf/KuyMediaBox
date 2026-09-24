package pdf

import (
	"bytes"
	"encoding/binary"
	"math"
)

// srgbProfile builds a small ICC v2 sRGB display profile (D50 PCS, Bradford-adapted
// primaries, 1024-point sRGB tone curve) for PDF/A output intents.
func srgbProfile() []byte {
	type tag struct {
		sig  string
		data []byte
	}
	s15 := func(v float64) []byte {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(int32(math.Round(v*65536))))
		return b
	}
	xyz := func(x, y, z float64) []byte {
		b := []byte("XYZ \x00\x00\x00\x00")
		b = append(b, s15(x)...)
		b = append(b, s15(y)...)
		return append(b, s15(z)...)
	}
	desc := func(s string) []byte {
		var b bytes.Buffer
		b.WriteString("desc\x00\x00\x00\x00")
		_ = binary.Write(&b, binary.BigEndian, uint32(len(s)+1))
		b.WriteString(s)
		b.WriteByte(0)
		b.Write(make([]byte, 4+4+2+1+67)) // empty Unicode and ScriptCode parts
		return b.Bytes()
	}
	text := func(s string) []byte { return append([]byte("text\x00\x00\x00\x00"+s), 0) }
	curve := func() []byte {
		var b bytes.Buffer
		b.WriteString("curv\x00\x00\x00\x00")
		const n = 1024
		_ = binary.Write(&b, binary.BigEndian, uint32(n))
		for i := 0; i < n; i++ {
			v := float64(i) / (n - 1)
			var l float64
			if v <= 0.04045 {
				l = v / 12.92
			} else {
				l = math.Pow((v+0.055)/1.055, 2.4)
			}
			_ = binary.Write(&b, binary.BigEndian, uint16(math.Round(l*65535)))
		}
		return b.Bytes()
	}()
	tags := []tag{
		{"desc", desc("sRGB IEC61966-2.1")},
		{"cprt", text("No copyright, use freely")},
		{"wtpt", xyz(0.9642, 1.0, 0.8249)},
		{"rXYZ", xyz(0.4361, 0.2225, 0.0139)},
		{"gXYZ", xyz(0.3851, 0.7169, 0.0971)},
		{"bXYZ", xyz(0.1431, 0.0606, 0.7141)},
		{"rTRC", curve},
		{"gTRC", nil}, // shares the red curve
		{"bTRC", nil},
	}
	pad4 := func(n int) int { return (n + 3) &^ 3 }
	offset := 128 + 4 + 12*len(tags)
	var table, data bytes.Buffer
	var curveOff, curveLen int
	for _, t := range tags {
		off, l := offset+data.Len(), len(t.data)
		if t.data == nil {
			off, l = curveOff, curveLen
		} else {
			if t.sig == "rTRC" {
				curveOff, curveLen = off, l
			}
			data.Write(t.data)
			data.Write(make([]byte, pad4(l)-l))
		}
		table.WriteString(t.sig)
		_ = binary.Write(&table, binary.BigEndian, uint32(off))
		_ = binary.Write(&table, binary.BigEndian, uint32(l))
	}
	size := offset + data.Len()
	h := make([]byte, 128)
	binary.BigEndian.PutUint32(h[0:], uint32(size))
	binary.BigEndian.PutUint32(h[8:], 0x02100000) // version 2.1
	copy(h[12:], "mntr")
	copy(h[16:], "RGB ")
	copy(h[20:], "XYZ ")
	binary.BigEndian.PutUint16(h[24:], 2026) // date/time
	binary.BigEndian.PutUint16(h[26:], 1)
	binary.BigEndian.PutUint16(h[28:], 1)
	copy(h[36:], "acsp")
	copy(h[68:], s15(0.9642)) // PCS illuminant D50
	copy(h[72:], s15(1.0))
	copy(h[76:], s15(0.8249))
	var out bytes.Buffer
	out.Write(h)
	_ = binary.Write(&out, binary.BigEndian, uint32(len(tags)))
	out.Write(table.Bytes())
	out.Write(data.Bytes())
	return out.Bytes()
}
