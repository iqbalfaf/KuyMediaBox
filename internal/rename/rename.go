// Package rename builds new file names for the bulk rename page: from a field written
// inside a document (report cards), a pattern with a counter, find & replace, or by adding
// and cutting text.
package rename

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/naming"
)

// Modes.
const (
	ModeContent = "content" // a field inside the PDF/Word document
	ModePattern = "pattern" // a pattern with {file}, {n}, {date}
	ModeReplace = "replace" // find & replace in the old name
	ModeEdit    = "edit"    // add a prefix/suffix, cut characters
)

// Options say how new names are built. Each mode reads its own fields; Case and Tidy
// apply to every mode.
type Options struct {
	Mode string `json:"mode"`

	Labels   string `json:"labels"`   // content: labels of the name field, comma separated, tried in order
	Template string `json:"template"` // content: e.g. "{nama}" or "{nama} - {nisn}"

	Pattern string `json:"pattern"` // pattern: e.g. "Foto {n}"
	Start   int    `json:"start"`   // first number of {n}
	Step    int    `json:"step"`    // added for each next file
	Digits  int    `json:"digits"`  // {n} padded with zeros to this width

	Find      string `json:"find"` // replace
	With      string `json:"with"`
	Regex     bool   `json:"regex"`
	MatchCase bool   `json:"matchCase"`

	Prefix   string `json:"prefix"` // edit (may use {n} and {date})
	Suffix   string `json:"suffix"`
	CutStart int    `json:"cutStart"` // characters removed at the start of the old name
	CutEnd   int    `json:"cutEnd"`   // … and at its end

	Case string `json:"case"` // keep | title | upper | lower (content mode: the name only)
	Tidy bool   `json:"tidy"` // underscores to spaces, single spaces
}

// DefaultLabels are the usual labels of the student name on Indonesian report cards.
const DefaultLabels = "Nama Peserta Didik, Nama Siswa, Nama Murid, Nama Lengkap, Nama"

// File is one file to rename.
type File struct {
	Name     string    // current name without extension
	Index    int       // position in the list (for {n})
	Modified time.Time // for {date}
	// Lines returns the document text (content mode only); "\t" separates columns.
	Lines func() ([]string, error)
}

// Built-in codes of the content template; any other {Label} looks that label up.
var fieldLabels = map[string][]string{
	"nisn":  {"NISN"},
	"nis":   {"NIS", "Nomor Induk", "No. Induk", "No Induk"},
	"kelas": {"Kelas"},
}

var (
	codeRe   = regexp.MustCompile(`\{([^{}]+)\}`)
	spaceRun = regexp.MustCompile(`\s+`)
)

// NewName returns the new name (without extension) of f.
func NewName(f File, o Options) (string, error) {
	var name string
	var err error
	switch o.Mode {
	case ModeContent:
		name, err = fromContent(f, o)
	case ModePattern:
		if strings.TrimSpace(o.Pattern) == "" {
			return "", errors.New(i18n.L("Isi pola nama", "Enter a name pattern"))
		}
		name = expand(o.Pattern, f, o)
	case ModeReplace:
		name, err = replace(f.Name, o)
	case ModeEdit:
		r := []rune(f.Name)
		cs, ce := max(0, o.CutStart), max(0, o.CutEnd)
		if cs+ce >= len(r) {
			return "", errors.New(i18n.L("Semua huruf nama lama terpotong", "Every character of the old name is cut"))
		}
		name = expand(o.Prefix, f, o) + string(r[cs:len(r)-ce]) + expand(o.Suffix, f, o)
	default:
		return "", fmt.Errorf("unknown mode %q", o.Mode)
	}
	if err != nil {
		return "", err
	}
	if o.Tidy {
		name = strings.ReplaceAll(name, "_", " ")
	}
	if o.Mode != ModeContent {
		name = applyCase(name, o.Case)
	}
	name = strings.TrimSpace(spaceRun.ReplaceAllString(name, " "))
	if name == "" {
		return "", errors.New(i18n.L("Nama baru kosong", "The new name is empty"))
	}
	return naming.SanitizeFileName(name), nil
}

// expand fills the codes every mode knows: {file}, {n} and {date}. Other text, braces
// included, stays as it is.
func expand(tpl string, f File, o Options) string {
	return codeRe.ReplaceAllStringFunc(tpl, func(m string) string {
		if v, ok := common(m, f, o); ok {
			return v
		}
		return m
	})
}

func common(m string, f File, o Options) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(m[1 : len(m)-1])) {
	case "file":
		return f.Name, true
	case "n", "no":
		step := o.Step
		if step == 0 {
			step = 1
		}
		n := o.Start + f.Index*step
		s := strconv.Itoa(abs(n))
		if d := min(o.Digits, 10); len(s) < d {
			s = strings.Repeat("0", d-len(s)) + s
		}
		if n < 0 {
			s = "-" + s
		}
		return s, true
	case "date", "tanggal":
		if f.Modified.IsZero() {
			return time.Now().Format("2006-01-02"), true
		}
		return f.Modified.Format("2006-01-02"), true
	}
	return "", false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func fromContent(f File, o Options) (string, error) {
	tpl := strings.TrimSpace(o.Template)
	if tpl == "" {
		tpl = "{nama}"
	}
	var lines []string
	var readErr error
	read := false
	var missing []string
	name := codeRe.ReplaceAllStringFunc(tpl, func(m string) string {
		if v, ok := common(m, f, o); ok {
			return v
		}
		if readErr != nil {
			return ""
		}
		if !read {
			read = true
			if f.Lines == nil {
				readErr = errors.New(i18n.L("Hanya untuk file PDF dan Word", "Only for PDF and Word files"))
			} else {
				lines, readErr = f.Lines()
			}
			if readErr != nil {
				return ""
			}
		}
		code := strings.TrimSpace(m[1 : len(m)-1])
		key := strings.ToLower(code)
		var labels []string
		switch {
		case key == "nama" || key == "name":
			if labels = SplitLabels(o.Labels); len(labels) == 0 {
				labels = SplitLabels(DefaultLabels)
			}
		case fieldLabels[key] != nil:
			labels = fieldLabels[key]
		default:
			labels = SplitLabels(code)
		}
		v := FindField(lines, labels)
		if v == "" {
			missing = append(missing, labels[0])
			return ""
		}
		if key == "nama" || key == "name" {
			// Only the name: codes like "XI IPS 2" stay as written.
			v = applyCase(v, o.Case)
		}
		return v
	})
	if readErr != nil {
		return "", readErr
	}
	if len(missing) > 0 {
		return "", fmt.Errorf(i18n.L("“%s” tidak ditemukan di dokumen", "“%s” was not found in the document"), strings.Join(missing, "”, “"))
	}
	return name, nil
}

func replace(name string, o Options) (string, error) {
	if o.Find == "" {
		return "", errors.New(i18n.L("Isi teks yang dicari", "Enter the text to find"))
	}
	expr := o.Find
	if !o.Regex {
		expr = regexp.QuoteMeta(expr)
	}
	if !o.MatchCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return "", errors.New(i18n.L("Pola regex tidak valid", "Invalid regex pattern"))
	}
	with := o.With
	if !o.Regex {
		with = strings.ReplaceAll(with, "$", "$$")
	}
	return re.ReplaceAllString(name, with), nil
}

// FindField returns the value written after the first label found (labels are tried in
// order), e.g. "Nama Peserta Didik : BUDI SANTOSO" → "BUDI SANTOSO". The label must be
// followed by a colon or the next column, so "Nama" doesn't match "Nama Sekolah".
func FindField(lines []string, labels []string) string {
	for _, label := range labels {
		words := strings.Fields(label)
		if len(words) == 0 {
			continue
		}
		for i, w := range words {
			words[i] = regexp.QuoteMeta(w)
		}
		re, err := regexp.Compile(`(?i)(?:^|[^\p{L}\p{N}])` + strings.Join(words, `[ \t]+`) + `(?:[ \t._]*[:：]|[ ]*\t)[ \t]*([^\t]*)`)
		if err != nil {
			continue
		}
		for _, l := range lines {
			for _, m := range re.FindAllStringSubmatch(l, -1) {
				if v := cleanValue(m[1]); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// cleanValue trims a field value; a value running into the next "Label : …" on the same
// line is cut before that label.
func cleanValue(v string) string {
	if i := strings.IndexAny(v, ":："); i >= 0 {
		f := strings.Fields(v[:i])
		if len(f) > 0 {
			f = f[:len(f)-1]
		}
		v = strings.Join(f, " ")
	}
	return strings.TrimFunc(v, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(".,;-_|", r)
	})
}

// SplitLabels turns "Nama Peserta Didik, Nama Siswa" into its labels.
func SplitLabels(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' || r == '\n' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func applyCase(s, mode string) string {
	switch mode {
	case "upper":
		return strings.ToUpper(s)
	case "lower":
		return strings.ToLower(s)
	case "title":
		r := []rune(strings.ToLower(s))
		for i := range r {
			if i == 0 || strings.ContainsRune(" -'’(._", r[i-1]) {
				r[i] = unicode.ToUpper(r[i])
			}
		}
		return string(r)
	}
	return s
}
