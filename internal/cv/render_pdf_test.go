package cv

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pdfNow is the fixed date every typst test renders at, so expiry is stable.
var pdfNow = time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)

// TestRenderTypstGolden pins the generated typst source, not the PDF bytes.
// Rewrite the files with:
//
//	go test ./internal/cv -run TestRenderTypstGolden -update
func TestRenderTypstGolden(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			got, err := RenderTypst(goldenDocument(lang), lang, pdfNow)
			if err != nil {
				t.Fatalf("RenderTypst() error = %v", err)
			}

			path := filepath.Join("testdata", "render", "full."+lang+".golden.typ")
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden file: %v (run with -update to create it)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from %s, run with -update and review the diff\n--- got\n%s\n--- want\n%s", path, got, want)
			}
		})
	}
}

func TestRenderTypstExpiry(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		validUntil string
		want       string
		notWant    string
	}{
		"past is expired":            {validUntil: "2026-01", want: "expired Jan 2026", notWant: "valid until"},
		"previous month is expired":  {validUntil: "2026-10", want: "expired Oct 2026", notWant: "valid until"},
		"current month is still due": {validUntil: "2026-11", want: "valid until Nov 2026", notWant: "expired"},
		"future is valid":            {validUntil: "2030-05", want: "valid until May 2030", notWant: "expired"},
		"none prints no status":      {validUntil: "", notWant: "valid until"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := Document{Certificates: []CertificateEntry{
				{ID: "c", Name: "Cert", Issuer: "Body", Issued: "2023-01", ValidUntil: tc.validUntil},
			}}
			got, err := RenderTypst(doc, "en", pdfNow)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && !strings.Contains(string(got), tc.want) {
				t.Errorf("source lacks %q:\n%s", tc.want, got)
			}
			if strings.Contains(string(got), tc.notWant) {
				t.Errorf("source contains %q:\n%s", tc.notWant, got)
			}
		})
	}

	t.Run("german wording", func(t *testing.T) {
		t.Parallel()

		doc := Document{Certificates: []CertificateEntry{
			{ID: "c", Name: "Zert", Issuer: "Stelle", Issued: "2023-01", ValidUntil: "2026-01"},
		}}
		got, err := RenderTypst(doc, "de", pdfNow)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "abgelaufen Jan. 2026") {
			t.Errorf("source lacks the German expiry:\n%s", got)
		}
	})
}

func TestRenderTypstOmitsEmptySections(t *testing.T) {
	t.Parallel()

	doc := Document{
		Experience: []ExperienceEntry{{ID: "e", Employer: "Acme", Title: "Engineer", Start: "2020-01"}},
		Contact:    Contact{Email: "a@example.com"},
	}
	got, err := RenderTypst(doc, "en", pdfNow)
	if err != nil {
		t.Fatal(err)
	}
	src := string(got)
	if n := strings.Count(src, "#heading("); n != 1 {
		t.Errorf("headings = %d, want 1 (experience only):\n%s", n, src)
	}
	if !strings.Contains(src, `#heading(level: 1, "Experience")`) {
		t.Errorf("experience heading missing:\n%s", src)
	}
}

func TestRenderTypstEscapesData(t *testing.T) {
	t.Parallel()

	doc := Document{Experience: []ExperienceEntry{{
		ID: "e", Employer: "A", Title: `Say "hi" \ #set page(width: 1pt) $x$`, Start: "2020-01",
		Summary: "line one\n\n  line two",
	}}}
	got, err := RenderTypst(doc, "en", pdfNow)
	if err != nil {
		t.Fatal(err)
	}
	src := string(got)
	for _, want := range []string{
		`"Say \"hi\" \\ #set page(width: 1pt) $x$"`,
		`body: "line one line two"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source lacks %s:\n%s", want, src)
		}
	}
}

func TestRenderTypstFailsLoudly(t *testing.T) {
	t.Parallel()

	good := Document{Experience: []ExperienceEntry{{ID: "e", Employer: "A", Title: "T", Start: "2020-01"}}}

	t.Run("a blank required text", func(t *testing.T) {
		t.Parallel()

		doc := good
		doc.Experience = []ExperienceEntry{{ID: "e", Employer: "A", Title: " \n", Start: "2020-01"}}
		if _, err := RenderTypst(doc, "en", pdfNow); err == nil || !strings.Contains(err.Error(), "blank") {
			t.Errorf("error = %v, want a blank text error", err)
		}
	})

	t.Run("a template field the model does not hold", func(t *testing.T) {
		t.Parallel()

		_, err := renderTypst(`{{s .Name}}{{s .NoSuchField}}`, good, "en", pdfNow)
		if err == nil || !strings.Contains(err.Error(), "NoSuchField") {
			t.Errorf("error = %v, want one naming NoSuchField", err)
		}
	})

	t.Run("a template syntax error", func(t *testing.T) {
		t.Parallel()

		if _, err := renderTypst(`{{if}}`, good, "en", pdfNow); err == nil {
			t.Error("error = nil, want a parse error")
		}
	})

	t.Run("an unknown language", func(t *testing.T) {
		t.Parallel()

		if _, err := RenderTypst(good, "fr", pdfNow); err == nil {
			t.Error("error = nil, want an unknown language error")
		}
	})

	t.Run("an engagement of an unknown experience", func(t *testing.T) {
		t.Parallel()

		doc := good
		doc.Engagements = []EngagementEntry{{ID: "g", Experience: "missing", Title: "T", Summary: "S"}}
		if _, err := RenderTypst(doc, "en", pdfNow); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Errorf("error = %v, want one naming the experience", err)
		}
	})

	t.Run("an unknown skill level", func(t *testing.T) {
		t.Parallel()

		doc := good
		doc.Skills = []SkillEntry{{ID: "s", Name: "Go", Level: "guru"}}
		if _, err := RenderTypst(doc, "en", pdfNow); err == nil || !strings.Contains(err.Error(), "guru") {
			t.Errorf("error = %v, want one naming the level", err)
		}
	})
}

// TestRenderTypstPrintsEverySection proves each model section reaches the
// source, by a marker text unique to its entries.
func TestRenderTypstPrintsEverySection(t *testing.T) {
	t.Parallel()

	got, err := RenderTypst(goldenDocument("en"), "en", pdfNow)
	if err != nil {
		t.Fatal(err)
	}
	src := string(got)
	for _, want := range []string{
		"Summary one en.", "Degree summary en.", "Grade 0,0 en", "Certificate en",
		"Expert", "Project text en.", "Example Person", "Fluent", "Volunteer text en.",
		"Award text en.", "Exam text en.", "Course en", "Article text en.", "Patent text en.",
		"Example Society en", "Engagement text en.", "Example Industry en",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source lacks %q", want)
		}
	}
}

func TestPDFFileName(t *testing.T) {
	t.Parallel()

	if got := PDFFileName("de"); got != "cv.de.pdf" {
		t.Errorf("PDFFileName(de) = %q", got)
	}
}
