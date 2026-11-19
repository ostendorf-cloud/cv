package cv

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update rewrites the golden files from the current renderer output:
//
//	go test ./internal/cv -run TestRenderJSONGolden -update
//
// Review the diff of testdata/render/*.golden.json before committing it: a
// change there is a change to the public contract, and may need a new
// SchemaVersion. See README.md.
var update = flag.Bool("update", false, "rewrite the golden files")

// pad surrounds every value of the padded fixture, including the unset
// optional ones, with whitespace the renderer has to remove again.
func pad(s string) string { return " \t" + s + " \n\n" }

func plain(s string) string { return s }

// fixtureDocument is a fake document with every section. Every optional
// field is set on one entry and unset on another, in each language: the
// address carries a street and a city in de and a city only in en. Each
// language carries its own text, so a mixup of languages shows too. Every
// string goes through p, which is plain for the golden fixture and pad for
// the whitespace fixture.
func fixtureDocument(lang string, p func(string) string) Document {
	doc := legacyFixture(lang, p)
	doc.Experience[0].Location = p("Example City " + lang)
	doc.Experience[0].EmploymentType = p("Full-time " + lang)
	doc.Experience[1].Location = p("")
	doc.Experience[1].EmploymentType = p("")
	doc.Education[0].Grade = p("0,0 " + lang)
	doc.Education[1].Grade = p("")
	// A certificate without an expiry renders valid_until as null.
	doc.Certificates = append(doc.Certificates, CertificateEntry{
		ID: p("cert-2"), Name: p("Lifetime Certificate " + lang), Issuer: p("Other Body " + lang), Issued: p("2021-05"), ValidUntil: p(""),
	})
	doc.Volunteering = []VolunteeringEntry{
		{ID: p("vol-1"), Organisation: p("Example Association " + lang), Role: p("Helper " + lang), Cause: p("Education " + lang), Start: p("2019-03"), End: p("2021-06"), Summary: p("Volunteer text " + lang + ".")},
		{ID: p("vol-2"), Organisation: p("Other Association " + lang), Role: p("Lead " + lang), Cause: p(""), Start: p("2021-07"), End: p(""), Summary: p("")},
	}
	doc.Awards = []AwardEntry{
		{ID: p("award-1"), Title: p("Award " + lang), Issuer: p("Example Body " + lang), Date: p("2020-11"), Summary: p("Award text " + lang + ".")},
		{ID: p("award-2"), Title: p("Other Award " + lang), Issuer: p("Other Body " + lang), Date: p("2021-12"), Summary: p("")},
	}
	doc.ExamResults = []ExamResultEntry{
		{ID: p("exam-1"), Name: p("Thesis " + lang), Score: p("0,0"), Date: p("2019-06"), Summary: p("Exam text " + lang + ".")},
		{ID: p("exam-2"), Name: p("Test " + lang), Score: p("passed"), Date: p("2018-02"), Summary: p("")},
	}
	doc.Courses = []CourseEntry{
		{ID: p("course-1"), Name: p("Course " + lang), Provider: p("Example Academy " + lang), Date: p("2022-02")},
		{ID: p("course-2"), Name: p("Other Course " + lang), Provider: p(""), Date: p("")},
	}
	doc.Publications = []PublicationEntry{
		{ID: p("pub-1"), Title: p("Article " + lang), Publisher: p("Example Journal " + lang), Date: p("2021-09"), URL: p("https://example.com/article"), Summary: p("Article text " + lang + ".")},
		{ID: p("pub-2"), Title: p("Other Article " + lang), Publisher: p(""), Date: p("2022-10"), URL: p(""), Summary: p("")},
	}
	doc.Patents = []PatentEntry{
		{ID: p("patent-1"), Title: p("Patent " + lang), Number: p("EX 0000000"), Date: p("2020-05"), URL: p("https://example.com/patent"), Summary: p("Patent text " + lang + ".")},
		{ID: p("patent-2"), Title: p("Other Patent " + lang), Number: p(""), Date: p("2020-06"), URL: p(""), Summary: p("")},
	}
	doc.Organisations = []OrganisationEntry{
		{ID: p("org-1"), Name: p("Example Society " + lang), Role: p("Member " + lang), Start: p("2018-01"), End: p("2020-12")},
		{ID: p("org-2"), Name: p("Other Society " + lang), Role: p(""), Start: p("2021-01"), End: p("")},
	}
	doc.Engagements = []EngagementEntry{
		{ID: p("eng-1"), Experience: p("exp-1"), Title: p("Example Engagement " + lang), Industry: p("Example Industry " + lang), Start: p("2020-03"), End: p("2021"), Summary: p("Engagement text " + lang + ".")},
		{ID: p("eng-2"), Experience: p("exp-2"), Title: p("Other Engagement " + lang), Industry: p(""), Start: p(""), End: p(""), Summary: p("Other engagement text " + lang + ".")},
	}
	return doc
}

// legacyDocument is the fixture as it was before the LinkedIn parity
// sections and fields existed. It stays so TestRenderJSONLegacyKeysUnchanged
// can prove the old part of the contract did not move.
func legacyDocument(lang string) Document { return legacyFixture(lang, plain) }

// legacyFixture is the fixture of the sections that existed before the
// LinkedIn parity change, every string going through p.
func legacyFixture(lang string, p func(string) string) Document {
	doc := Document{
		Experience: []ExperienceEntry{
			{ID: p("exp-1"), Employer: p("Example Employer " + lang), Title: p("Engineer " + lang), Start: p("2020-01"), End: p("2022-01"), Summary: p("Summary one " + lang + ".")},
			{ID: p("exp-2"), Employer: p("Other Employer " + lang), Title: p("Lead " + lang), Start: p("2022-02"), End: p(""), Summary: p("")},
		},
		Education: []EducationEntry{
			{ID: p("edu-1"), Institution: p("Example University " + lang), Degree: p("Degree " + lang), Start: p("2015-10"), End: p("2019-06"), Summary: p("Degree summary " + lang + ".")},
			{ID: p("edu-2"), Institution: p("Other School " + lang), Degree: p("Course " + lang), Start: p("2019-09"), End: p(""), Summary: p("")},
		},
		Certificates: []CertificateEntry{
			{ID: p("cert-1"), Name: p("Certificate " + lang), Issuer: p("Example Body " + lang), Issued: p("2023-01"), ValidUntil: p("2026-01")},
		},
		Skills: []SkillEntry{
			{ID: p("skill-1"), Name: p("Kubernetes " + lang), Level: Level(p(string(LevelExpert))), Evidence: []Evidence{
				{Type: EvidenceType(p(string(EvidenceTypeCertificate))), Ref: p("cert-1")},
				{Type: EvidenceType(p(string(EvidenceTypeProject))), Ref: p("proj-1")},
			}},
			{ID: p("skill-2"), Name: p("Go " + lang), Level: Level(p(string(LevelWorking)))},
		},
		Projects: []ProjectEntry{
			{ID: p("proj-1"), GithubRepo: p("example/repo"), Description: p("Project text " + lang + ".")},
		},
		References: []ReferenceEntry{
			{ID: p("ref-1"), Name: p("Example Person"), Role: p("Manager " + lang)},
		},
		Languages: []LanguageEntry{
			{ID: p("lang-1"), Name: p("English " + lang), Level: Level(p(string(LevelProficient)))},
			{ID: p("lang-2"), Name: p("German " + lang), Level: Level(p(string(LevelExpert)))},
		},
		Contact: Contact{
			Email: p("contact@example.com"),
			Profiles: []Profile{
				{ID: p("github"), Platform: p("GitHub"), URL: p("https://github.com/example")},
			},
		},
	}
	if lang == "de" {
		doc.Contact.Address = &Address{Street: p("Example Street 1"), City: p("Example City")}
	} else {
		doc.Contact.Address = &Address{Street: p(""), City: p("Example City")}
	}
	return doc
}

func goldenDocument(lang string) Document { return fixtureDocument(lang, plain) }

func TestRenderJSONGolden(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			got, err := RenderJSON(goldenDocument(lang), lang)
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}

			path := filepath.Join("testdata", "render", "full."+lang+".golden.json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
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

// TestRenderJSONTrimsEveryField proves the contract: every text field has its
// leading and trailing whitespace removed, and a whitespace only optional
// value is null. The fixture carries padding in every string field, so the
// output must be byte for byte the golden file of the unpadded fixture.
func TestRenderJSONTrimsEveryField(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			padded, err := RenderJSON(fixtureDocument(lang, pad), lang)
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "render", "full."+lang+".golden.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(padded, want) {
				t.Errorf("padded input does not render as the golden file\n--- got\n%s\n--- want\n%s", padded, want)
			}
		})
	}
}

func TestRenderJSONSummaryWhitespace(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		summary string
		want    string
	}{
		"a folded yaml newline is trimmed":     {summary: "Text.\n", want: `"summary": "Text."`},
		"trailing spaces and tabs are trimmed": {summary: "Text. \t\r\n\n", want: `"summary": "Text."`},
		"leading whitespace is trimmed":        {summary: "  Text.", want: `"summary": "Text."`},
		"inner newlines stay":                  {summary: "One.\nTwo.\n", want: `"summary": "One.\nTwo."`},
		"whitespace only becomes null":         {summary: " \n", want: `"summary": null`},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := Document{
				Experience: []ExperienceEntry{{ID: "e", Employer: "E", Title: "T", Start: "2020-01", Summary: tc.summary}},
				Contact:    Contact{Email: "contact@example.com"},
			}
			data, err := RenderJSON(doc, "en")
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			if !strings.Contains(string(data), tc.want) {
				t.Errorf("output lacks %s:\n%s", tc.want, data)
			}
		})
	}
}

// TestRenderJSONDoesNotChangeItsInput guards the trim: it must work on the
// output, never on the caller's document.
func TestRenderJSONDoesNotChangeItsInput(t *testing.T) {
	t.Parallel()

	doc := fixtureDocument("en", pad)
	if _, err := RenderJSON(doc, "en"); err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	if got, want := doc.Experience[0].Employer, pad("Example Employer en"); got != want {
		t.Errorf("Employer = %q, want %q", got, want)
	}
	if got, want := doc.Skills[0].Evidence[0].Ref, pad("cert-1"); got != want {
		t.Errorf("Evidence ref = %q, want %q", got, want)
	}
}
