package cv

import (
	"strings"
	"testing"
)

// validDocument is a fake document with every section filled, which passes
// Validate. Each test below blanks exactly one value of it.
func validDocument() Document {
	return Document{
		Experience:   []ExperienceEntry{{ID: "exp-1", Employer: "Acme", Title: "Engineer", Start: "2020-01"}},
		Education:    []EducationEntry{{ID: "edu-1", Institution: "University", Degree: "BSc", Start: "2016-10"}},
		Certificates: []CertificateEntry{{ID: "cert-1", Name: "CKAD", Issuer: "Body", Issued: "2024-05", ValidUntil: "2027-05"}},
		Skills: []SkillEntry{{
			ID: "skill-1", Name: "Kubernetes", Level: LevelExpert,
			Evidence: []Evidence{{Type: EvidenceTypeProject, Ref: "proj-1"}},
		}},
		Projects:   []ProjectEntry{{ID: "proj-1", GithubRepo: "acme/cv", Description: "A CV"}},
		References: []ReferenceEntry{{ID: "ref-1", Name: "Jane Doe", Role: "Manager"}},
		Languages:  []LanguageEntry{{ID: "lang-1", Name: "German", Level: LevelExpert}},
		Contact: Contact{
			Email:    "contact@example.com",
			Profiles: []Profile{{ID: "github", Platform: "GitHub", URL: "https://github.com/example"}},
		},
	}
}

// TestValidateRejectsBlankValues proves a whitespace only value fails like an
// empty one, in every section and for every required field. Without it, a
// description of " \n" would pass here and render as an empty string.
func TestValidateRejectsBlankValues(t *testing.T) {
	t.Parallel()

	const blank = " \t\n"

	tests := map[string]struct {
		blank func(d *Document)
		want  string
	}{
		"experience id":            {func(d *Document) { d.Experience[0].ID = blank }, "id: is required"},
		"experience employer":      {func(d *Document) { d.Experience[0].Employer = blank }, "employer: is required"},
		"experience title":         {func(d *Document) { d.Experience[0].Title = blank }, "title: is required"},
		"experience start":         {func(d *Document) { d.Experience[0].Start = blank }, "start: is required"},
		"education id":             {func(d *Document) { d.Education[0].ID = blank }, "id: is required"},
		"education institution":    {func(d *Document) { d.Education[0].Institution = blank }, "institution: is required"},
		"education degree":         {func(d *Document) { d.Education[0].Degree = blank }, "degree: is required"},
		"education start":          {func(d *Document) { d.Education[0].Start = blank }, "start: is required"},
		"certificate id":           {func(d *Document) { d.Certificates[0].ID = blank }, "id: is required"},
		"certificate name":         {func(d *Document) { d.Certificates[0].Name = blank }, "name: is required"},
		"certificate issuer":       {func(d *Document) { d.Certificates[0].Issuer = blank }, "issuer: is required"},
		"certificate issued":       {func(d *Document) { d.Certificates[0].Issued = blank }, "issued: is required"},
		"certificate valid_until":  {func(d *Document) { d.Certificates[0].ValidUntil = blank }, "valid_until: is required"},
		"skill id":                 {func(d *Document) { d.Skills[0].ID = blank }, "id: is required"},
		"skill name":               {func(d *Document) { d.Skills[0].Name = blank }, "name: is required"},
		"skill level":              {func(d *Document) { d.Skills[0].Level = Level(blank) }, "level: is required"},
		"skill evidence ref":       {func(d *Document) { d.Skills[0].Evidence[0].Ref = blank }, "ref: is required"},
		"project id":               {func(d *Document) { d.Projects[0].ID = blank }, "id: is required"},
		"project github_repo":      {func(d *Document) { d.Projects[0].GithubRepo = blank }, "github_repo: is required"},
		"project description":      {func(d *Document) { d.Projects[0].Description = blank }, "description: is required"},
		"reference id":             {func(d *Document) { d.References[0].ID = blank }, "id: is required"},
		"reference name":           {func(d *Document) { d.References[0].Name = blank }, "name: is required"},
		"reference role":           {func(d *Document) { d.References[0].Role = blank }, "role: is required"},
		"language id":              {func(d *Document) { d.Languages[0].ID = blank }, "id: is required"},
		"language name":            {func(d *Document) { d.Languages[0].Name = blank }, "name: is required"},
		"language level":           {func(d *Document) { d.Languages[0].Level = Level(blank) }, "level: is required"},
		"contact email":            {func(d *Document) { d.Contact.Email = blank }, "email: is required"},
		"contact profile id":       {func(d *Document) { d.Contact.Profiles[0].ID = blank }, "id: is required"},
		"contact profile platform": {func(d *Document) { d.Contact.Profiles[0].Platform = blank }, "platform: is required"},
		"contact profile url":      {func(d *Document) { d.Contact.Profiles[0].URL = blank }, "url: is required"},
	}

	if err := Validate(validDocument(), "test"); err != nil {
		t.Fatalf("the base document must be valid: %v", err)
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := validDocument()
			tc.blank(&doc)
			err := Validate(doc, "test")
			if err == nil {
				t.Fatalf("Validate() error = nil, want one containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestValidateBlankIdsAreNotDuplicates(t *testing.T) {
	t.Parallel()

	doc := validDocument()
	doc.Skills = []SkillEntry{
		{ID: " ", Name: "A", Level: LevelWorking},
		{ID: " ", Name: "B", Level: LevelWorking},
	}
	err := Validate(doc, "test")
	if err == nil {
		t.Fatal("Validate() error = nil, want the missing ids reported")
	}
	if strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Validate() error = %v, blank ids must not also count as duplicates", err)
	}
}
