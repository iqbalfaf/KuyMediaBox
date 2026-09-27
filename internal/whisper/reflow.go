package whisper

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Cue is one subtitle: start and end in seconds.
type Cue struct {
	Start, End float64
	Text       string
}

var (
	reCueTime  = regexp.MustCompile(`(\d+):(\d+):(\d+)[,.](\d+)\s*-->\s*(\d+):(\d+):(\d+)[,.](\d+)`)
	reCueBlank = regexp.MustCompile(`\r?\n\s*\r?\n`)
	bom        = string(rune(0xFEFF))
)

func stamp(h, m, s, ms string) float64 {
	hh, _ := strconv.Atoi(h)
	mm, _ := strconv.Atoi(m)
	ss, _ := strconv.Atoi(s)
	frac, _ := strconv.Atoi((ms + "000")[:3])
	return float64(hh*3600+mm*60+ss) + float64(frac)/1000
}

// ParseCues reads SRT or WebVTT text.
func ParseCues(text string) []Cue {
	var out []Cue
	for _, b := range reCueBlank.Split(strings.TrimSpace(strings.TrimPrefix(text, bom)), -1) {
		lines := strings.Split(strings.ReplaceAll(b, "\r\n", "\n"), "\n")
		for i, l := range lines {
			m := reCueTime.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			if body := strings.Join(strings.Fields(strings.Join(lines[i+1:], " ")), " "); body != "" {
				out = append(out, Cue{Start: stamp(m[1], m[2], m[3], m[4]), End: stamp(m[5], m[6], m[7], m[8]), Text: body})
			}
			break
		}
	}
	return out
}

// Reflow makes cues readable: at most two lines of maxLen characters each. Longer cues are
// split at word boundaries, their time shared out by length; a very short leftover line
// joins the line before it when that stays close to maxLen.
func Reflow(cues []Cue, maxLen int) []Cue {
	if maxLen <= 0 {
		return cues
	}
	runes := utf8.RuneCountInString
	var out []Cue
	for _, c := range cues {
		// Fill lines greedily.
		var lines []string
		cur := ""
		for _, w := range strings.Fields(c.Text) {
			if next := strings.TrimSpace(cur + " " + w); cur == "" || runes(next) <= maxLen {
				cur = next
				continue
			}
			lines = append(lines, cur)
			cur = w
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		if len(lines) == 0 {
			continue
		}
		if n := len(lines); n > 1 && runes(lines[n-1]) < maxLen/3 && runes(lines[n-2])+1+runes(lines[n-1]) <= maxLen*6/5 {
			lines[n-2] += " " + lines[n-1]
			lines = lines[:n-1]
		}
		// Two lines per cue.
		var chunks []string
		for i := 0; i < len(lines); i += 2 {
			if i+1 < len(lines) {
				chunks = append(chunks, lines[i]+" "+lines[i+1])
			} else {
				chunks = append(chunks, lines[i])
			}
		}
		total := 0
		for _, ch := range chunks {
			total += runes(ch)
		}
		t := c.Start
		for i, ch := range chunks {
			end := c.End
			if i < len(chunks)-1 {
				end = t + (c.End-c.Start)*float64(runes(ch))/float64(max(total, 1))
			}
			out = append(out, Cue{Start: t, End: end, Text: wrap(ch, maxLen)})
			t = end
		}
	}
	return out
}

// wrap breaks text into one or two lines: the most even split whose lines both fit, or
// failing that the one that overflows least.
func wrap(text string, maxLen int) string {
	if utf8.RuneCountInString(text) <= maxLen {
		return text
	}
	words := strings.Fields(text)
	best, bestScore := text, 1<<30
	for i := 1; i < len(words); i++ {
		a, b := strings.Join(words[:i], " "), strings.Join(words[i:], " ")
		la, lb := utf8.RuneCountInString(a), utf8.RuneCountInString(b)
		score := abs(la - lb)
		if over := max(la, lb) - maxLen; over > 0 {
			score += 1000 * over
		}
		if score < bestScore {
			best, bestScore = a+"\n"+b, score
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clockSRT(t float64, sep string) string {
	ms := int(t*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", ms/3600000, ms/60000%60, ms/1000%60, sep, ms%1000)
}

// FormatSRT writes cues as SRT.
func FormatSRT(cues []Cue) string {
	var b strings.Builder
	for i, c := range cues {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, clockSRT(c.Start, ","), clockSRT(c.End, ","), c.Text)
	}
	return b.String()
}

// FormatVTT writes cues as WebVTT.
func FormatVTT(cues []Cue) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, c := range cues {
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", clockSRT(c.Start, "."), clockSRT(c.End, "."), c.Text)
	}
	return b.String()
}

// ReflowFile rewrites an .srt or .vtt file in place with Reflow.
func ReflowFile(path string, maxLen int) error {
	if maxLen <= 0 {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cues := Reflow(ParseCues(string(data)), maxLen)
	if len(cues) == 0 {
		return nil
	}
	body := FormatSRT(cues)
	if strings.HasSuffix(strings.ToLower(path), ".vtt") {
		body = FormatVTT(cues)
	}
	return os.WriteFile(path, []byte(body), 0o644)
}
