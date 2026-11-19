package cv

import (
	"strings"
	"testing"
)

func TestValidateNewSections(t *testing.T) {
	t.Parallel()

	if err := Validate(goldenDocument("en"), "test"); err != nil {
		t.Fatalf("the golden document must be valid: %v", err)
	}

	tests := map[string]struct {
		mutate func(*Document)
		want   string
	}{
		"volunteering without organisation":    {func(d *Document) { d.Volunteering[0].Organisation = "" }, "volunteering[0] (id=vol-1): organisation: is required"},
		"volunteering without role":            {func(d *Document) { d.Volunteering[0].Role = "" }, "role: is required"},
		"volunteering without start":           {func(d *Document) { d.Volunteering[0].Start = "" }, "start: is required"},
		"volunteering without id":              {func(d *Document) { d.Volunteering[0].ID = "" }, "volunteering[0]: id: is required"},
		"volunteering bad start":               {func(d *Document) { d.Volunteering[0].Start = "2019" }, `start: "2019" is not a YYYY-MM date`},
		"volunteering bad end":                 {func(d *Document) { d.Volunteering[0].End = "2021-13" }, `end: "2021-13" is not a YYYY-MM date`},
		"volunteering duplicate id":            {func(d *Document) { d.Volunteering[1].ID = "vol-1" }, `volunteering: duplicate id "vol-1"`},
		"volunteering end before start":        {func(d *Document) { d.Volunteering[0].End = "2019-02" }, `end: "2019-02" is before start "2019-03"`},
		"organisations end before start":       {func(d *Document) { d.Organisations[0].End = "2017-12" }, `end: "2017-12" is before start "2018-01"`},
		"awards without title":                 {func(d *Document) { d.Awards[0].Title = "" }, "title: is required"},
		"awards without issuer":                {func(d *Document) { d.Awards[0].Issuer = "" }, "issuer: is required"},
		"awards without date":                  {func(d *Document) { d.Awards[0].Date = "" }, "date: is required"},
		"awards bad date":                      {func(d *Document) { d.Awards[0].Date = "2020-11-05" }, "is not a YYYY-MM date"},
		"awards duplicate id":                  {func(d *Document) { d.Awards[1].ID = "award-1" }, `awards: duplicate id "award-1"`},
		"exam results without name":            {func(d *Document) { d.ExamResults[0].Name = "" }, "name: is required"},
		"exam results without score":           {func(d *Document) { d.ExamResults[0].Score = "" }, "score: is required"},
		"exam results without date":            {func(d *Document) { d.ExamResults[0].Date = "" }, "date: is required"},
		"exam results bad date":                {func(d *Document) { d.ExamResults[0].Date = "June 2019" }, "is not a YYYY-MM date"},
		"exam results duplicate id":            {func(d *Document) { d.ExamResults[1].ID = "exam-1" }, `exam_results: duplicate id "exam-1"`},
		"courses without name":                 {func(d *Document) { d.Courses[0].Name = "" }, "name: is required"},
		"courses bad date":                     {func(d *Document) { d.Courses[0].Date = "22-02" }, "is not a YYYY-MM date"},
		"courses duplicate id":                 {func(d *Document) { d.Courses[1].ID = "course-1" }, `courses: duplicate id "course-1"`},
		"publications without title":           {func(d *Document) { d.Publications[0].Title = "" }, "title: is required"},
		"publications without date":            {func(d *Document) { d.Publications[0].Date = "" }, "date: is required"},
		"publications bad date":                {func(d *Document) { d.Publications[0].Date = "2021-00" }, "is not a YYYY-MM date"},
		"publications duplicate id":            {func(d *Document) { d.Publications[1].ID = "pub-1" }, `publications: duplicate id "pub-1"`},
		"patents without title":                {func(d *Document) { d.Patents[0].Title = "" }, "title: is required"},
		"patents without date":                 {func(d *Document) { d.Patents[0].Date = "" }, "date: is required"},
		"patents bad date":                     {func(d *Document) { d.Patents[0].Date = "2020/05" }, "is not a YYYY-MM date"},
		"patents duplicate id":                 {func(d *Document) { d.Patents[1].ID = "patent-1" }, `patents: duplicate id "patent-1"`},
		"organisations without name":           {func(d *Document) { d.Organisations[0].Name = "" }, "name: is required"},
		"organisations without start":          {func(d *Document) { d.Organisations[0].Start = "" }, "start: is required"},
		"organisations bad start":              {func(d *Document) { d.Organisations[0].Start = "2018-1" }, "is not a YYYY-MM date"},
		"organisations bad end":                {func(d *Document) { d.Organisations[0].End = "x" }, "is not a YYYY-MM date"},
		"organisations duplicate id":           {func(d *Document) { d.Organisations[1].ID = "org-1" }, `organisations: duplicate id "org-1"`},
		"engagements without id":               {func(d *Document) { d.Engagements[0].ID = "" }, "engagements[0]: id: is required"},
		"engagements without experience":       {func(d *Document) { d.Engagements[0].Experience = "" }, "engagements[0] (id=eng-1): experience: is required"},
		"engagements without title":            {func(d *Document) { d.Engagements[0].Title = "" }, "title: is required"},
		"engagements without summary":          {func(d *Document) { d.Engagements[0].Summary = "" }, "summary: is required"},
		"engagements unknown experience":       {func(d *Document) { d.Engagements[0].Experience = "exp-9" }, `experience: no experience with id "exp-9" exists`},
		"engagements bad start":                {func(d *Document) { d.Engagements[0].Start = "2020-3" }, `start: "2020-3" is not a YYYY-MM or YYYY date`},
		"engagements bad end":                  {func(d *Document) { d.Engagements[0].End = "2021-13" }, `end: "2021-13" is not a YYYY-MM or YYYY date`},
		"engagements full date":                {func(d *Document) { d.Engagements[0].Start = "2020-03-01" }, "is not a YYYY-MM or YYYY date"},
		"engagements end before start":         {func(d *Document) { d.Engagements[0].End = "2020-02" }, `end: "2020-02" is before start "2020-03"`},
		"engagements end year before start":    {func(d *Document) { d.Engagements[0].End = "2019" }, `end: "2019" is before start "2020-03"`},
		"engagements start year after end":     {func(d *Document) { d.Engagements[0].Start, d.Engagements[0].End = "2022", "2021-12" }, `end: "2021-12" is before start "2022"`},
		"engagements padded end before start":  {func(d *Document) { d.Engagements[0].Start, d.Engagements[0].End = " 2021 ", "2020-06\n" }, `end: "2020-06" is before start "2021"`},
		"volunteering padded end before start": {func(d *Document) { d.Volunteering[0].Start, d.Volunteering[0].End = " 2019-03 ", "2019-02\n" }, `end: "2019-02" is before start "2019-03"`},
		"engagements duplicate id":             {func(d *Document) { d.Engagements[1].ID = "eng-1" }, `engagements: duplicate id "eng-1"`},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := goldenDocument("en")
			tc.mutate(&doc)
			err := Validate(doc, "test")
			if err == nil {
				t.Fatal("Validate() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestValidateEndInStartMonth proves an end in the start month is valid.
func TestValidateEndInStartMonth(t *testing.T) {
	t.Parallel()

	doc := goldenDocument("en")
	doc.Volunteering[0].End = doc.Volunteering[0].Start
	doc.Organisations[0].End = doc.Organisations[0].Start
	if err := Validate(doc, "test"); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

// TestValidateEngagementDates proves the accepted date forms and orders.
func TestValidateEngagementDates(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ start, end string }{
		"both unset":              {"", ""},
		"start only":              {"2020-03", ""},
		"end only":                {"", "2021-06"},
		"months":                  {"2020-03", "2021-06"},
		"years":                   {"2020", "2021"},
		"same year":               {"2020", "2020"},
		"same month":              {"2020-03", "2020-03"},
		"start year, end month":   {"2020", "2020-01"},
		"start month, end year":   {"2020-12", "2020"},
		"padded":                  {" 2020-03\n", "\t2021 "},
		"year start, month later": {"2020", "2021-01"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := goldenDocument("en")
			doc.Engagements[0].Start, doc.Engagements[0].End = tc.start, tc.end
			if err := Validate(doc, "test"); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestValidateWithoutEngagements proves the section is optional: a document
// without it validates, and a missing list is the same as an empty one.
func TestValidateWithoutEngagements(t *testing.T) {
	t.Parallel()

	for name, engagements := range map[string][]EngagementEntry{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := goldenDocument("en")
			doc.Engagements = engagements
			if err := Validate(doc, "test"); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
	if err := Validate(legacyDocument("en"), "test"); err != nil {
		t.Fatalf("Validate() of a document without engagements = %v, want nil", err)
	}
}

// TestValidateOptionalNewFieldsMayBeEmpty proves the optional fields are not
// required.
func TestValidateOptionalNewFieldsMayBeEmpty(t *testing.T) {
	t.Parallel()

	doc := Document{
		Contact:       Contact{Email: "contact@example.com"},
		Volunteering:  []VolunteeringEntry{{ID: "v", Organisation: "O", Role: "R", Start: "2020-01"}},
		Awards:        []AwardEntry{{ID: "a", Title: "T", Issuer: "I", Date: "2020-01"}},
		ExamResults:   []ExamResultEntry{{ID: "x", Name: "N", Score: "1", Date: "2020-01"}},
		Courses:       []CourseEntry{{ID: "c", Name: "N"}},
		Publications:  []PublicationEntry{{ID: "p", Title: "T", Date: "2020-01"}},
		Patents:       []PatentEntry{{ID: "t", Title: "T", Date: "2020-01"}},
		Organisations: []OrganisationEntry{{ID: "o", Name: "N", Start: "2020-01"}},
		Engagements:   []EngagementEntry{{ID: "g", Experience: "e", Title: "T", Summary: "S"}},
	}
	doc.Experience = []ExperienceEntry{{ID: "e", Employer: "E", Title: "T", Start: "2020-01"}}
	if err := Validate(doc, "test"); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

// TestDriftNewSections checks, for every new section, an id only in the
// German file and an id only in the English one.
func TestDriftNewSections(t *testing.T) {
	t.Parallel()

	drop := map[string]func(*Document){
		"volunteering":  func(d *Document) { d.Volunteering = d.Volunteering[:1] },
		"awards":        func(d *Document) { d.Awards = d.Awards[:1] },
		"exam_results":  func(d *Document) { d.ExamResults = d.ExamResults[:1] },
		"courses":       func(d *Document) { d.Courses = d.Courses[:1] },
		"publications":  func(d *Document) { d.Publications = d.Publications[:1] },
		"patents":       func(d *Document) { d.Patents = d.Patents[:1] },
		"organisations": func(d *Document) { d.Organisations = d.Organisations[:1] },
		"engagements":   func(d *Document) { d.Engagements = d.Engagements[:1] },
	}

	if err := Drift(goldenDocument("de"), goldenDocument("en")); err != nil {
		t.Fatalf("Drift() on matching documents = %v, want nil", err)
	}

	for section, fn := range drop {
		t.Run(section+" only in de", func(t *testing.T) {
			t.Parallel()

			en := goldenDocument("en")
			fn(&en)
			err := Drift(goldenDocument("de"), en)
			if err == nil || !strings.Contains(err.Error(), section) || !strings.Contains(err.Error(), "German file but not the English") {
				t.Fatalf("Drift() error = %v, want a German only drift in %s", err, section)
			}
		})
		t.Run(section+" only in en", func(t *testing.T) {
			t.Parallel()

			de := goldenDocument("de")
			fn(&de)
			err := Drift(de, goldenDocument("en"))
			if err == nil || !strings.Contains(err.Error(), section) || !strings.Contains(err.Error(), "English file but not the German") {
				t.Fatalf("Drift() error = %v, want an English only drift in %s", err, section)
			}
		})
	}
}

const newSectionsYAML = `
contact:
  email: contact@example.com
experience:
  - {id: e, employer: E, title: T, start: "2020-01", location: Berlin, employment_type: Full-time}
education:
  - {id: d, institution: I, degree: D, start: "2020-01", grade: "0,0"}
volunteering:
  - {id: v, organisation: O, role: R, cause: C, start: "2020-01", end: null}
awards:
  - {id: a, title: T, issuer: I, date: "2020-01"}
exam_results:
  - {id: x, name: N, score: "0,0", date: "2020-01"}
courses:
  - {id: c, name: N}
publications:
  - {id: p, title: T, date: "2020-01", url: https://example.com}
patents:
  - {id: t, title: T, date: "2020-01", number: EX 1}
organisations:
  - {id: o, name: N, start: "2020-01"}
engagements:
  - {id: g, experience: e, title: T, industry: I, start: "2020", end: "2021-02", summary: S}
`

func TestParseNewSections(t *testing.T) {
	t.Parallel()

	doc, err := Parse("test", []byte(newSectionsYAML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if doc.Education[0].Grade != "0,0" || doc.Experience[0].Location != "Berlin" ||
		doc.Experience[0].EmploymentType != "Full-time" || doc.ExamResults[0].Score != "0,0" ||
		len(doc.Volunteering) != 1 || len(doc.Awards) != 1 || len(doc.Courses) != 1 ||
		len(doc.Publications) != 1 || len(doc.Patents) != 1 || len(doc.Organisations) != 1 ||
		len(doc.Engagements) != 1 || doc.Engagements[0].End != "2021-02" {
		t.Errorf("Parse() decoded %+v", doc)
	}
}

func TestParseRejectsUnknownFieldInNewSection(t *testing.T) {
	t.Parallel()

	data := "contact: {email: contact@example.com}\nawards:\n  - {id: a, title: T, issuer: I, date: \"2020-01\", rank: first}\n"
	if _, err := Parse("test", []byte(data)); err == nil {
		t.Fatal("Parse() error = nil, want an unknown field error")
	}
}
