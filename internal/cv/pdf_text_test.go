package cv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// realTool returns the path of a binary the real PDF tests need. Outside CI a
// missing binary skips the test. In CI it fails it: the test job installs
// typst with scripts/install-typst.sh and pdftotext with apt, so a missing
// one is a broken job, not a reason to pass.
func realTool(t *testing.T, name string) string {
	t.Helper()

	path, err := exec.LookPath(name)
	if err == nil {
		// A binary that cannot print its version is as good as missing,
		// such as a mise shim without a version for this directory.
		versionFlag := "-v"
		if name == "typst" {
			versionFlag = "--version"
		}
		err = exec.CommandContext(t.Context(), path, versionFlag).Run()
	}
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is not usable on PATH: the CI test job must install it: %v", name, err)
		}
		t.Skipf("%s is not usable on PATH, skipped outside CI: %v", name, err)
	}
	return path
}

// compileReal renders doc as typst source and compiles it with the real
// typst, without a photo.
func compileReal(t *testing.T, doc Document, lang string) []byte {
	t.Helper()

	typst := realTool(t, "typst")
	source, err := RenderTypst(doc, lang, TypstOptions{Now: pdfNow})
	if err != nil {
		t.Fatalf("RenderTypst() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pdf, err := CompilePDF(ctx, typst, source, nil)
	if err != nil {
		t.Fatalf("CompilePDF() error = %v", err)
	}
	return pdf
}

// pdfText is the text layer of pdf as pdftotext reads it, the way an
// applicant tracking system reads it, with every whitespace run collapsed to
// one space.
func pdfText(t *testing.T, pdf []byte) string {
	t.Helper()

	pdftotext := realTool(t, "pdftotext")
	path := filepath.Join(t.TempDir(), "cv.pdf")
	if err := os.WriteFile(path, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), pdftotext, "-enc", "UTF-8", path, "-").Output()
	if err != nil {
		t.Fatalf("pdftotext: %v", err)
	}
	return strings.Join(strings.Fields(string(out)), " ")
}

// TestPDFTextLayer checks that the PDF's text extracts as it reads: headings
// as whole words, and a hyphen as the ASCII hyphen a search for "E-Mail"
// matches, not as letter spaced capitals or U+2010.
func TestPDFTextLayer(t *testing.T) {
	t.Parallel()

	doc := Document{
		Experience: []ExperienceEntry{{
			ID: "now", Employer: "Acme", Title: "DevOps- und Cloud-Engineer", Start: "2020-01",
			Summary: "Betrieb des E-Mail-Versands.",
		}},
		Education: []EducationEntry{{ID: "u", Institution: "Uni", Degree: "BSc", Start: "2015-10", End: "2019-09"}},
		Contact:   Contact{Email: "a@example.com"},
	}

	cases := []struct {
		name string
		lang string
		want []string
	}{
		{
			name: "german headings and hyphens",
			lang: "de",
			want: []string{"BERUFSERFAHRUNG", "AUSBILDUNG", "E-MAIL", "DevOps- und Cloud-Engineer", "E-Mail-Versands"},
		},
		{
			name: "english headings",
			lang: "en",
			want: []string{"EXPERIENCE", "EDUCATION", "EMAIL", "CURRICULUM VITAE"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			text := pdfText(t, compileReal(t, doc, tc.lang))
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Errorf("text layer lacks %q:\n%s", want, text)
				}
			}
			if strings.ContainsRune(text, '\u2010') {
				t.Errorf("text layer holds U+2010 instead of the ASCII hyphen:\n%s", text)
			}
		})
	}
}
