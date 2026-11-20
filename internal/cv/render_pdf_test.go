package cv

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pdfNow is the fixed date every typst test renders at, so expiry and the
// footer's date line are stable.
var pdfNow = time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)

// pdfOpts renders at pdfNow with a photo, as the release does.
var pdfOpts = TypstOptions{Now: pdfNow, Photo: true}

func renderSource(t *testing.T, doc Document, lang string) string {
	t.Helper()
	got, err := RenderTypst(doc, lang, pdfOpts)
	if err != nil {
		t.Fatalf("RenderTypst() error = %v", err)
	}
	return string(got)
}

// TestRenderTypstGolden pins the generated typst source, not the PDF bytes.
// Rewrite the files with:
//
//	go test ./internal/cv -run TestRenderTypstGolden -update
func TestRenderTypstGolden(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			got, err := RenderTypst(goldenDocument(lang), lang, pdfOpts)
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
		"past is expired":            {validUntil: "2026-01", want: `marker: "expired Jan 2026"`, notWant: "valid until"},
		"previous month is expired":  {validUntil: "2026-10", want: `marker: "expired Oct 2026"`, notWant: "valid until"},
		"current month is still due": {validUntil: "2026-11", want: `org: "Body · valid until Nov 2026"`, notWant: `marker: "`},
		"future is valid":            {validUntil: "2030-05", want: `org: "Body · valid until May 2030"`, notWant: `marker: "`},
		"none prints no status":      {validUntil: "", want: `org: "Body")`, notWant: "valid until"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := Document{Certificates: []CertificateEntry{
				{ID: "c", Name: "Cert", Issuer: "Body", Issued: "2023-01", ValidUntil: tc.validUntil},
			}}
			src := renderSource(t, doc, "en")
			if !strings.Contains(src, tc.want) {
				t.Errorf("source lacks %q:\n%s", tc.want, src)
			}
			if strings.Contains(src, tc.notWant) {
				t.Errorf("source contains %q:\n%s", tc.notWant, src)
			}
		})
	}

	t.Run("german wording", func(t *testing.T) {
		t.Parallel()

		doc := Document{Certificates: []CertificateEntry{
			{ID: "c", Name: "Zert", Issuer: "Stelle", Issued: "2023-01", ValidUntil: "2026-01"},
		}}
		if src := renderSource(t, doc, "de"); !strings.Contains(src, `marker: "abgelaufen Jan. 2026"`) {
			t.Errorf("source lacks the German expiry marker:\n%s", src)
		}
	})
}

// TestRenderTypstLanguageLevels pins the wording of each language level, in
// both files.
func TestRenderTypstLanguageLevels(t *testing.T) {
	t.Parallel()

	want := map[string]map[Level]string{
		"de": {
			LevelExpert: "Muttersprache", LevelProficient: "Fließend",
			LevelWorking: "Gute Kenntnisse", LevelFamiliar: "Grundkenntnisse",
		},
		"en": {
			LevelExpert: "Native", LevelProficient: "Fluent",
			LevelWorking: "Good working knowledge", LevelFamiliar: "Basic",
		},
	}
	for lang, words := range want {
		for level, word := range words {
			t.Run(lang+"/"+string(level), func(t *testing.T) {
				t.Parallel()

				doc := Document{
					Languages: []LanguageEntry{{ID: "l", Name: "Tongue", Level: level}},
					Contact:   Contact{Email: "a@example.com"},
				}
				if src := renderSource(t, doc, lang); !strings.Contains(src, `"Tongue (`+word+`)"`) {
					t.Errorf("source lacks %q:\n%s", "Tongue ("+word+")", src)
				}
			})
		}
	}

	t.Run("every level has a word in every language", func(t *testing.T) {
		t.Parallel()

		for lang, l := range pdfLabels {
			for _, level := range levels {
				if l.LangLevels[level] == "" {
					t.Errorf("%s: no word for %s", lang, level)
				}
			}
		}
	})

	t.Run("an unknown level fails", func(t *testing.T) {
		t.Parallel()

		doc := Document{Languages: []LanguageEntry{{ID: "l", Name: "Tongue", Level: "guru"}}}
		if _, err := RenderTypst(doc, "en", pdfOpts); err == nil || !strings.Contains(err.Error(), "guru") {
			t.Errorf("error = %v, want one naming the level", err)
		}
	})
}

// TestRenderTypstSkillCategories proves the grouping: categories in display
// order, skills inside by level, highest first, equal levels in file order,
// and an empty category left out.
func TestRenderTypstSkillCategories(t *testing.T) {
	t.Parallel()

	one := 1
	doc := Document{
		SkillCategories: []SkillCategoryEntry{
			{ID: "cloud", Name: "Cloud"},
			{ID: "empty", Name: "Nobody Here"},
			{ID: "code", Name: "Code", Order: &one},
		},
		Skills: []SkillEntry{
			{ID: "a", Name: "Ansible", Level: LevelWorking, Category: "cloud"},
			{ID: "b", Name: "Bash", Level: LevelFamiliar, Category: "code"},
			{ID: "k", Name: "Kubernetes", Level: LevelExpert, Category: "cloud"},
			{ID: "g", Name: "Go", Level: LevelProficient, Category: "code"},
			{ID: "h", Name: "Helm", Level: LevelWorking, Category: "cloud"},
		},
	}
	src := renderSource(t, doc, "en")

	for _, want := range []string{
		`category("Code", ("Go", "Bash", ))`,
		`category("Cloud", ("Kubernetes", "Ansible", "Helm", ))`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source lacks %s:\n%s", want, src)
		}
	}
	if strings.Index(src, `category("Code"`) > strings.Index(src, `category("Cloud"`) {
		t.Errorf("Code (order 1) does not come before Cloud:\n%s", src)
	}
	if strings.Contains(src, "Nobody Here") {
		t.Errorf("the empty category is printed:\n%s", src)
	}
	for _, level := range []string{"expert", "proficient", "Expert", "Proficient"} {
		if strings.Contains(src, level) {
			t.Errorf("source prints the level %q:\n%s", level, src)
		}
	}

	t.Run("no skill at all leaves the section out", func(t *testing.T) {
		t.Parallel()

		doc := Document{SkillCategories: []SkillCategoryEntry{{ID: "empty", Name: "Nobody Here"}}}
		if src := renderSource(t, doc, "en"); strings.Contains(src, "#section(") || strings.Contains(src, "Nobody") {
			t.Errorf("source prints a skills section without skills:\n%s", src)
		}
	})

	t.Run("an unknown category fails", func(t *testing.T) {
		t.Parallel()

		doc := Document{Skills: []SkillEntry{{ID: "s", Name: "Go", Level: LevelExpert, Category: "missing"}}}
		if _, err := RenderTypst(doc, "en", pdfOpts); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Errorf("error = %v, want one naming the category", err)
		}
	})
}

func TestRenderTypstOmitsEmptySections(t *testing.T) {
	t.Parallel()

	doc := Document{
		Experience: []ExperienceEntry{{ID: "e", Employer: "Acme", Title: "Engineer", Start: "2020-01"}},
		Contact:    Contact{Email: "a@example.com"},
	}
	src := renderSource(t, doc, "en")
	if n := strings.Count(src, "#section("); n != 1 {
		t.Errorf("sections = %d, want 1 (experience only):\n%s", n, src)
	}
	if !strings.Contains(src, `#section("Experience")`) {
		t.Errorf("experience section missing:\n%s", src)
	}
}

// TestRenderTypstHeader covers the header: the headline, the info row and
// the footer's date line.
func TestRenderTypstHeader(t *testing.T) {
	t.Parallel()

	doc := Document{
		Experience: []ExperienceEntry{
			{ID: "old", Employer: "Old", Title: "Past Role", Start: "2018-01", End: "2019-01"},
			{ID: "now", Employer: "Acme", Title: "Platform Engineer", Start: "2020-01"},
			{ID: "side", Employer: "Self", Title: "Side Role", Start: "2021-01"},
		},
		Contact: Contact{Email: "a@example.com", Address: &Address{City: "Aachen"}},
	}

	t.Run("the headline is the first ongoing position", func(t *testing.T) {
		t.Parallel()

		src := renderSource(t, doc, "en")
		if !strings.Contains(src, `text(size: 11.5pt, fill: muted, "Platform Engineer")`) {
			t.Errorf("headline is not the first ongoing title:\n%s", src)
		}
	})

	t.Run("no ongoing position prints no headline", func(t *testing.T) {
		t.Parallel()

		past := Document{Experience: doc.Experience[:1], Contact: doc.Contact}
		if src := renderSource(t, past, "en"); strings.Contains(src, "size: 11.5pt") {
			t.Errorf("source prints a headline:\n%s", src)
		}
	})

	t.Run("the info row holds what the data has", func(t *testing.T) {
		t.Parallel()

		src := renderSource(t, doc, "de")
		for _, want := range []string{
			`info("Ort", ("Aachen", ))`,
			`info("E-Mail", (link("mailto:a@example.com", "a@example.com"), ))`,
			`columns: (1fr,) * 2`,
		} {
			if !strings.Contains(src, want) {
				t.Errorf("source lacks %s:\n%s", want, src)
			}
		}
		for _, notWant := range []string{"Telefon", "Phone", "Sprachen", "Profile"} {
			if strings.Contains(src, notWant) {
				t.Errorf("source prints %q without data for it:\n%s", notWant, src)
			}
		}
	})

	t.Run("the footer carries the render month", func(t *testing.T) {
		t.Parallel()

		for lang, want := range map[string]string{"de": `"Stand: November 2026"`, "en": `"As of November 2026"`} {
			if src := renderSource(t, doc, lang); !strings.Contains(src, want) {
				t.Errorf("%s: source lacks %s", lang, want)
			}
		}
		march := TypstOptions{Now: time.Date(2027, time.March, 1, 0, 0, 0, 0, time.UTC)}
		got, err := RenderTypst(doc, "de", march)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), `"Stand: März 2027"`) {
			t.Errorf("source lacks the March date line:\n%s", got)
		}
	})

	t.Run("without a photo the header has one column", func(t *testing.T) {
		t.Parallel()

		got, err := RenderTypst(doc, "en", TypstOptions{Now: pdfNow})
		if err != nil {
			t.Fatal(err)
		}
		src := string(got)
		if strings.Contains(src, "image(") || !strings.Contains(src, "columns: (1fr,),") {
			t.Errorf("source without a photo still places one:\n%s", src)
		}
	})
}

func TestRenderTypstEscapesData(t *testing.T) {
	t.Parallel()

	doc := Document{Experience: []ExperienceEntry{{
		ID: "e", Employer: "A", Title: `Say "hi" \ #set page(width: 1pt) $x$`, Start: "2020-01",
		Summary: "line one\n\n  line two",
	}}}
	src := renderSource(t, doc, "en")
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
		if _, err := RenderTypst(doc, "en", pdfOpts); err == nil || !strings.Contains(err.Error(), "blank") {
			t.Errorf("error = %v, want a blank text error", err)
		}
	})

	t.Run("a template field the model does not hold", func(t *testing.T) {
		t.Parallel()

		_, err := renderTypst(`{{s .Name}}{{s .NoSuchField}}`, good, "en", pdfOpts)
		if err == nil || !strings.Contains(err.Error(), "NoSuchField") {
			t.Errorf("error = %v, want one naming NoSuchField", err)
		}
	})

	t.Run("a template syntax error", func(t *testing.T) {
		t.Parallel()

		if _, err := renderTypst(`{{if}}`, good, "en", pdfOpts); err == nil {
			t.Error("error = nil, want a parse error")
		}
	})

	t.Run("an unknown language", func(t *testing.T) {
		t.Parallel()

		if _, err := RenderTypst(good, "fr", pdfOpts); err == nil {
			t.Error("error = nil, want an unknown language error")
		}
	})

	t.Run("an engagement of an unknown experience", func(t *testing.T) {
		t.Parallel()

		doc := good
		doc.Engagements = []EngagementEntry{{ID: "g", Experience: "missing", Title: "T", Summary: "S"}}
		if _, err := RenderTypst(doc, "en", pdfOpts); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Errorf("error = %v, want one naming the experience", err)
		}
	})

	t.Run("an unknown skill level", func(t *testing.T) {
		t.Parallel()

		doc := good
		doc.SkillCategories = []SkillCategoryEntry{{ID: "c", Name: "C"}}
		doc.Skills = []SkillEntry{{ID: "s", Name: "Go", Level: "guru", Category: "c"}}
		if _, err := RenderTypst(doc, "en", pdfOpts); err == nil || !strings.Contains(err.Error(), "guru") {
			t.Errorf("error = %v, want one naming the level", err)
		}
	})
}

// TestRenderTypstPrintsEverySection proves each model section reaches the
// source, by a marker text unique to its entries.
func TestRenderTypstPrintsEverySection(t *testing.T) {
	t.Parallel()

	src := renderSource(t, goldenDocument("en"), "en")
	for _, want := range []string{
		"Summary one en.", "Degree summary en.", "Grade 0,0 en", "Certificate en",
		"Kubernetes en", "Project text en.", "Example Person", "Fluent", "Volunteer text en.",
		"Award text en.", "Exam text en.", "Course en", "Article text en.", "Patent text en.",
		"Example Society en", "Engagement text en.", "Example Industry en",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source lacks %q", want)
		}
	}
}

func TestPeriod(t *testing.T) {
	t.Parallel()

	l := pdfLabels["en"]
	tests := []struct {
		start, end string
		ongoing    bool
		want       string
	}{
		{"2020-01", "2021-03", true, "Jan 2020 – Mar 2021"},
		{"2020-01", "", true, "Jan 2020 – present"},
		{"2020-01", "", false, "Jan 2020"},
		{"2021-09", "2021-09", true, "Sep 2021"},
		{"2026", "2026", false, "2026"},
		{"", "2022-05", false, "May 2022"},
	}
	for _, tt := range tests {
		if got := l.period(tt.start, tt.end, tt.ongoing); got != tt.want {
			t.Errorf("period(%q, %q, %v) = %q, want %q", tt.start, tt.end, tt.ongoing, got, tt.want)
		}
	}
}

func TestLoadPhoto(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	t.Run("the repository portrait loads", func(t *testing.T) {
		t.Parallel()

		data, err := LoadPhoto("../../assets/portrait.jpg")
		if err != nil || len(data) == 0 {
			t.Errorf("LoadPhoto() = %d bytes, %v", len(data), err)
		}
	})

	t.Run("a missing file names the path", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(dir, "missing.jpg")
		_, err := LoadPhoto(missing)
		if err == nil || !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "portrait") {
			t.Errorf("error = %v, want one naming the portrait and %s", err, missing)
		}
	})

	t.Run("a file that is not a JPEG", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(dir, "text.jpg")
		if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPhoto(path); err == nil || !strings.Contains(err.Error(), "not a JPEG") {
			t.Errorf("error = %v, want a not a JPEG error", err)
		}
	})
}

// TestBundledFonts proves the font the template names ships with the binary.
func TestBundledFonts(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Go-Regular.ttf", "Go-Bold.ttf"} {
		data, err := fonts.ReadFile("templates/fonts/" + name)
		if err != nil || len(data) == 0 {
			t.Errorf("%s: %d bytes, %v", name, len(data), err)
		}
	}
	if !strings.Contains(typstTemplate, `font: "Go"`) {
		t.Error(`the template does not name the bundled font "Go"`)
	}
	if _, err := os.Stat(filepath.Join("templates", "fonts", "LICENSE")); err != nil {
		t.Errorf("the font licence is missing: %v", err)
	}
}

func TestPDFFileName(t *testing.T) {
	t.Parallel()

	if got := PDFFileName("de"); got != "cv.de.pdf" {
		t.Errorf("PDFFileName(de) = %q", got)
	}
}
