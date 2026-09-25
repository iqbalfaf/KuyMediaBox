package pdf

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

// VeraRule is one PDF/A rule a file breaks.
type VeraRule struct {
	Clause      string `json:"clause"`
	Spec        string `json:"spec"`
	Description string `json:"description"`
	Failed      int    `json:"failed"`
}

// VeraResult is veraPDF's verdict on a file.
type VeraResult struct {
	Compliant bool       `json:"compliant"`
	Profile   string     `json:"profile"` // e.g. "PDF/A-2b validation profile"
	Passed    int        `json:"passed"`  // rules passed
	Failed    []VeraRule `json:"failed"`
}

type mrrReport struct {
	Jobs []struct {
		Report struct {
			Profile   string `xml:"profileName,attr"`
			Compliant string `xml:"isCompliant,attr"`
			Details   struct {
				Passed int `xml:"passedRules,attr"`
				Rules  []struct {
					Spec        string `xml:"specification,attr"`
					Clause      string `xml:"clause,attr"`
					Status      string `xml:"status,attr"`
					Failed      int    `xml:"failedChecks,attr"`
					Description string `xml:"description"`
				} `xml:"rule"`
			} `xml:"details"`
		} `xml:"validationReport"`
		TaskException struct {
			Message string `xml:"exceptionMessage"`
		} `xml:"taskException"`
	} `xml:"jobs>job"`
}

// ValidatePDFA checks a file with veraPDF. bat is verapdf.bat (its folder holds the jars),
// java the java.exe to use; flavour is "0" (auto: what the file claims), "1b", "2b", "3b", "2u", …
func ValidatePDFA(ctx context.Context, bat, java, file, flavour string) (VeraResult, error) {
	if bat == "" {
		return VeraResult{}, errors.New(i18n.L("veraPDF belum terpasang. Pasang di Pengaturan › Tools pendukung.", "veraPDF is not installed. Install it in Settings › Helper tools."))
	}
	if java == "" {
		return VeraResult{}, errors.New(i18n.L("Java tidak ditemukan. Klik Unduh pada veraPDF di Pengaturan.", "Java was not found. Click Download next to veraPDF in Settings."))
	}
	switch flavour {
	case "1a", "1b", "2a", "2b", "2u", "3a", "3b", "3u", "4":
	default:
		flavour = "0"
	}
	home := filepath.Dir(bat)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	args := []string{
		"-classpath", filepath.Join(home, "etc") + ";" + filepath.Join(home, "bin", "*"),
		"-Dfile.encoding=UTF8", "-XX:+IgnoreUnrecognizedVMOptions",
		"-Dapp.home=" + home, "-Dbasedir=" + home, "-Dapp.repo=" + filepath.Join(home, "bin"),
		"--add-exports=java.base/sun.security.pkcs=ALL-UNNAMED",
		"org.verapdf.apps.GreenfieldCliWrapper", "--format", "mrr", "--flavour", flavour, file,
	}
	cmd := proc.Command(ctx, java, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return VeraResult{}, ctx.Err()
	}
	out := stdout.String()
	if i := strings.Index(out, "<?xml"); i > 0 {
		out = out[i:]
	}
	var rep mrrReport
	if err := xml.Unmarshal([]byte(out), &rep); err != nil || len(rep.Jobs) == 0 {
		detail := strings.TrimSpace(stderr.String())
		if runErr != nil && detail == "" {
			detail = runErr.Error()
		}
		return VeraResult{}, fmt.Errorf(i18n.L("veraPDF tidak memberi laporan: %s", "veraPDF gave no report: %s"), proc.LastLines(detail, 3))
	}
	job := rep.Jobs[0]
	if job.Report.Profile == "" && job.TaskException.Message != "" {
		return VeraResult{}, errors.New(i18n.L("veraPDF tidak bisa membaca file: ", "veraPDF can't read the file: ") + job.TaskException.Message)
	}
	res := VeraResult{Compliant: job.Report.Compliant == "true", Profile: job.Report.Profile, Passed: job.Report.Details.Passed}
	for _, r := range job.Report.Details.Rules {
		if r.Status == "failed" {
			res.Failed = append(res.Failed, VeraRule{Clause: r.Clause, Spec: r.Spec, Description: strings.TrimSpace(r.Description), Failed: r.Failed})
		}
	}
	return res, nil
}

// ShortProfile turns "PDF/A-2b validation profile" into "PDF/A-2b".
func (r VeraResult) ShortProfile() string {
	return strings.TrimSpace(strings.TrimSuffix(r.Profile, "validation profile"))
}

// Report is a readable list of the broken rules (for the task details).
func (r VeraResult) Report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", r.ShortProfile(), map[bool]string{true: "OK", false: "FAILED"}[r.Compliant])
	fmt.Fprintf(&b, i18n.L("Aturan lolos: %d, gagal: %d\n\n", "Rules passed: %d, failed: %d\n\n"), r.Passed, len(r.Failed))
	for _, f := range r.Failed {
		fmt.Fprintf(&b, "• %s %s (%d×): %s\n", f.Spec, f.Clause, f.Failed, f.Description)
	}
	return b.String()
}
