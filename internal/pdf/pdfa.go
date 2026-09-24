package pdf

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"kuymediabox/internal/i18n"
)

// PDFAResult says what the PDF/A conversion had to change.
type PDFAResult struct {
	RasterPages []int // pages turned into images because their fonts weren't embedded
}

// ToPDFA converts a document to PDF/A-2b (archival): it removes what the standard forbids
// (encryption, JavaScript, embedded files, forbidden actions), adds an sRGB output intent and
// XMP metadata, and turns pages that use non-embedded fonts into images.
func ToPDFA(ctx context.Context, in Input, out string, prog Progress) (PDFAResult, error) {
	var res PDFAResult
	pc, err := readCtx(in.Path, in.Password)
	if err != nil {
		return res, err
	}
	if pc.XRefTable.Version() >= model.V20 {
		return res, errors.New(i18n.L("PDF 2.0 belum bisa diubah ke PDF/A-2", "PDF 2.0 can't be converted to PDF/A-2 yet"))
	}
	xt := pc.XRefTable
	root, err := xt.Catalog()
	if err != nil {
		return res, err
	}

	// Fonts that are not embedded, and the pages that use them.
	bad := map[int]bool{}
	for nr, e := range xt.Table {
		if e == nil || e.Free || e.Object == nil {
			continue
		}
		if d, ok := e.Object.(types.Dict); ok && d.Type() != nil && *d.Type() == "Font" && !fontEmbedded(xt, d) {
			bad[nr] = true
		}
	}
	if len(bad) > 0 {
		doc, err := Open(ctx, in.Path, in.Password)
		if err != nil {
			return res, friendly(err)
		}
		defer doc.Close()
		for p := 1; p <= pc.PageCount; p++ {
			if err := ctx.Err(); err != nil {
				return res, err
			}
			page, _, err := pageGeom(pc, p)
			if err != nil {
				return res, err
			}
			_, _, inh, _ := pc.PageDict(p, false)
			resObj := types.Object(nil)
			if o, ok := page.Find("Resources"); ok {
				resObj = o
			} else if inh != nil {
				resObj = inh.Resources
			}
			if !usesFonts(xt, resObj, bad, map[int]bool{}, 0) {
				continue
			}
			img, err := doc.RenderDPI(p-1, 200)
			if err != nil {
				return res, err
			}
			if err := replaceWithImage(pc, p, flattenWhite(img)); err != nil {
				return res, err
			}
			res.RasterPages = append(res.RasterPages, p)
		}
	}
	prog(0.4)

	// Catalog clean-up.
	if names, err := xt.DereferenceDict(root["Names"]); err == nil && names != nil {
		names.Delete("JavaScript")
		names.Delete("EmbeddedFiles")
	}
	root.Delete("AA")
	root.Delete("NeedsRendering")
	if oa, err := xt.DereferenceDict(root["OpenAction"]); err == nil && oa != nil && forbiddenAction(oa) {
		root.Delete("OpenAction")
	}
	if af, err := xt.DereferenceDict(root["AcroForm"]); err == nil && af != nil {
		af.Delete("XFA")
		af.Update("NeedAppearances", types.Boolean(false))
	}

	// Objects everywhere: images, graphics states, streams, actions.
	for _, e := range xt.Table {
		if e == nil || e.Free || e.Object == nil {
			continue
		}
		switch v := e.Object.(type) {
		case types.StreamDict:
			if st := v.Subtype(); st != nil && *st == "Image" {
				v.Delete("Alternates")
				v.Delete("OPI")
				if b := v.BooleanEntry("Interpolate"); b != nil && *b {
					v.Update("Interpolate", types.Boolean(false))
				}
			}
			if st := v.Subtype(); st != nil && *st == "Form" {
				v.Delete("OPI")
			}
			for _, f := range v.FilterPipeline {
				if f.Name == "LZWDecode" {
					if err := v.Decode(); err == nil {
						v.FilterPipeline = []types.PDFFilter{{Name: "FlateDecode"}}
						v.Update("Filter", types.Name("FlateDecode"))
						v.Delete("DecodeParms")
						if err := v.Encode(); err == nil {
							l := int64(len(v.Raw))
							v.StreamLength = &l
							v.Update("Length", types.Integer(l))
						}
					}
					break
				}
			}
			e.Object = v
		case types.Dict:
			if t := v.Type(); t != nil && *t == "ExtGState" {
				if tr, ok := v.Find("TR"); ok {
					if n, isName := tr.(types.Name); !isName || n != "Default" {
						v.Delete("TR")
					}
				}
				v.Delete("TR2")
			}
			if v.Type() != nil && *v.Type() == "Page" {
				v.Delete("AA")
			}
		}
	}

	// Annotations must be printable, visible and have appearances.
	for p := 1; p <= pc.PageCount; p++ {
		page, _, err := pageGeom(pc, p)
		if err != nil {
			return res, err
		}
		arr, err := xt.DereferenceArray(page["Annots"])
		if err != nil || arr == nil {
			continue
		}
		var keep types.Array
		for _, o := range arr {
			a, err := xt.DereferenceDict(o)
			if err != nil || a == nil {
				continue
			}
			sub := ""
			if s := a.Subtype(); s != nil {
				sub = *s
			}
			if sub == "Popup" {
				keep = append(keep, o)
				continue
			}
			if _, hasAP := a.Find("AP"); !hasAP && sub != "Link" {
				continue // no appearance: drop
			}
			flags := 0
			if f := a.IntEntry("F"); f != nil {
				flags = *f
			}
			flags = (flags | 4) &^ (1 | 2 | 32 | 256) // Print on; Invisible, Hidden, NoView, ToggleNoView off
			a.Update("F", types.Integer(flags))
			if act, err := xt.DereferenceDict(a["A"]); err == nil && act != nil && forbiddenAction(act) {
				a.Delete("A")
			}
			a.Delete("AA")
			keep = append(keep, o)
		}
		page.Update("Annots", keep)
	}
	prog(0.6)

	// Output intent with an sRGB profile.
	icc, err := xt.NewStreamDictForBuf(srgbProfile())
	if err != nil {
		return res, err
	}
	icc.InsertInt("N", 3)
	if err := icc.Encode(); err != nil {
		return res, err
	}
	iccRef, err := xt.IndRefForNewObject(*icc)
	if err != nil {
		return res, err
	}
	oi := types.NewDict()
	oi.InsertName("Type", "OutputIntent")
	oi.InsertName("S", "GTS_PDFA1")
	oi.InsertString("OutputConditionIdentifier", "sRGB IEC61966-2.1")
	oi.InsertString("RegistryName", "http://www.color.org")
	oi.InsertString("Info", "sRGB IEC61966-2.1")
	oi.Insert("DestOutputProfile", *iccRef)
	root.Update("OutputIntents", types.Array{oi})

	// Fresh document info (pdfcpu fills Producer and dates) and matching XMP metadata.
	title := xt.Title
	info := types.NewDict()
	infoRef, err := xt.IndRefForNewObject(info)
	if err != nil {
		return res, err
	}
	xt.Info = infoRef
	// pdfcpu stamps the current second into the info dict while writing; start on a fresh
	// second so the XMP dates written here are identical.
	now := time.Now()
	if now.Nanosecond() > 700_000_000 {
		time.Sleep(time.Duration(1_000_000_000-now.Nanosecond()+20_000_000) * time.Nanosecond)
		now = time.Now()
	}
	xmp := xmpPacket(title, "pdfcpu "+model.VersionStr, now)
	meta := types.StreamDict{Dict: types.NewDict(), Raw: []byte(xmp), Content: []byte(xmp)}
	l := int64(len(xmp))
	meta.StreamLength = &l
	meta.InsertName("Type", "Metadata")
	meta.InsertName("Subtype", "XML")
	meta.InsertInt("Length", int(l))
	metaRef, err := xt.IndRefForNewObject(meta)
	if err != nil {
		return res, err
	}
	root.Update("Metadata", *metaRef)
	prog(0.8)
	return res, writeCtx(pc, out)
}

func forbiddenAction(a types.Dict) bool {
	s, _ := a["S"].(types.Name)
	switch s {
	case "Launch", "Sound", "Movie", "ResetForm", "ImportData", "JavaScript", "Hide", "SetOCGState", "Rendition", "Trans", "GoTo3DView":
		return true
	}
	return false
}

func fontEmbedded(xt *model.XRefTable, font types.Dict) bool {
	sub := ""
	if s := font.Subtype(); s != nil {
		sub = *s
	}
	switch sub {
	case "Type3":
		return true
	case "Type0":
		arr, err := xt.DereferenceArray(font["DescendantFonts"])
		if err != nil || len(arr) == 0 {
			return false
		}
		d, err := xt.DereferenceDict(arr[0])
		if err != nil || d == nil {
			return false
		}
		return fontEmbedded(xt, d)
	}
	fd, err := xt.DereferenceDict(font["FontDescriptor"])
	if err != nil || fd == nil {
		return false
	}
	for _, k := range []string{"FontFile", "FontFile2", "FontFile3"} {
		if _, ok := fd.Find(k); ok {
			return true
		}
	}
	return false
}

// usesFonts reports whether resources (or nested forms) refer to one of the bad fonts.
func usesFonts(xt *model.XRefTable, resObj types.Object, bad, seen map[int]bool, depth int) bool {
	if resObj == nil || depth > 6 {
		return false
	}
	res, err := xt.DereferenceDict(resObj)
	if err != nil || res == nil {
		return false
	}
	if fonts, err := xt.DereferenceDict(res["Font"]); err == nil {
		for _, o := range fonts {
			if ir, ok := o.(types.IndirectRef); ok && bad[ir.ObjectNumber.Value()] {
				return true
			}
		}
	}
	xo, err := xt.DereferenceDict(res["XObject"])
	if err != nil {
		return false
	}
	for _, o := range xo {
		ir, ok := o.(types.IndirectRef)
		if !ok || seen[ir.ObjectNumber.Value()] {
			continue
		}
		seen[ir.ObjectNumber.Value()] = true
		obj, err := xt.Dereference(ir)
		if err != nil {
			continue
		}
		if sd, ok := obj.(types.StreamDict); ok && sd.Subtype() != nil && *sd.Subtype() == "Form" {
			if usesFonts(xt, sd.Dict["Resources"], bad, seen, depth+1) {
				return true
			}
		}
	}
	return false
}

func xmpDate(t time.Time) string {
	_, off := t.Zone()
	sign := "+"
	if off < 0 {
		sign, off = "-", -off
	}
	return fmt.Sprintf("%s%s%02d:%02d", t.Format("2006-01-02T15:04:05"), sign, off/3600, off%3600/60)
}

func xmpPacket(title, producer string, t time.Time) string {
	var dc string
	if strings.TrimSpace(title) != "" {
		dc = fmt.Sprintf("\n   <dc:title><rdf:Alt><rdf:li xml:lang=\"x-default\">%s</rdf:li></rdf:Alt></dc:title>", html.EscapeString(title))
	}
	d := xmpDate(t)
	return `<?xpacket begin="` + string(rune(0xFEFF)) + `" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""
    xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/"
    xmlns:dc="http://purl.org/dc/elements/1.1/"
    xmlns:xmp="http://ns.adobe.com/xap/1.0/"
    xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
   <pdfaid:part>2</pdfaid:part>
   <pdfaid:conformance>B</pdfaid:conformance>
   <dc:format>application/pdf</dc:format>` + dc + `
   <xmp:CreateDate>` + d + `</xmp:CreateDate>
   <xmp:ModifyDate>` + d + `</xmp:ModifyDate>
   <xmp:MetadataDate>` + d + `</xmp:MetadataDate>
   <xmp:CreatorTool>KuyMediaBox</xmp:CreatorTool>
   <pdf:Producer>` + html.EscapeString(producer) + `</pdf:Producer>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`
}
