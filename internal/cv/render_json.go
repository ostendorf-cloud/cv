package cv

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// SchemaVersion is the version of the JSON document RenderJSON writes. It is
// a contract with every consumer: a consumer refuses a document whose
// schema_version it does not know. Raise it on any change a consumer reading
// the old shape would get wrong: a removed or renamed key, a changed type or
// a changed meaning. Adding a key a consumer can ignore does not need a
// raise, but it still needs a decision, so nothing else moves it.
const SchemaVersion = 1

// Languages is every language a document is rendered in, one per data file.
var Languages = []string{"de", "en"}

// JSONFileName is the release asset name of the JSON document for lang, for
// example cv.en.json. It is part of the contract with the consumers.
func JSONFileName(lang string) string {
	return "cv." + lang + ".json"
}

// jsonDocument is the public JSON shape. It is a type of its own, not the
// YAML model with tags, so that renaming a YAML key can never change the
// contract by accident. Where a type matches its YAML model field for field,
// it is converted, so a new model field breaks the build until someone
// decides whether it joins the contract. Key order is the field order below. Every
// list is always present, empty rather than null, and every optional scalar
// is always present, null when unset, so a consumer never probes for a key.
type jsonDocument struct {
	SchemaVersion int               `json:"schema_version"`
	Language      string            `json:"language"`
	Experience    []jsonExperience  `json:"experience"`
	Education     []jsonEducation   `json:"education"`
	Certificates  []jsonCertificate `json:"certificates"`
	Skills        []jsonSkill       `json:"skills"`
	Projects      []jsonProject     `json:"projects"`
	References    []jsonReference   `json:"references"`
	Languages     []jsonLanguage    `json:"languages"`
	Contact       jsonContact       `json:"contact"`

	// The sections below come after contact, so the keys above keep their
	// order.
	Volunteering  []jsonVolunteering `json:"volunteering"`
	Awards        []jsonAward        `json:"awards"`
	ExamResults   []jsonExamResult   `json:"exam_results"`
	Courses       []jsonCourse       `json:"courses"`
	Publications  []jsonPublication  `json:"publications"`
	Patents       []jsonPatent       `json:"patents"`
	Organisations []jsonOrganisation `json:"organisations"`
	Engagements   []jsonEngagement   `json:"engagements"`

	// SkillCategories comes last, so every key above keeps its place.
	SkillCategories []jsonSkillCategory `json:"skill_categories"`

	// Tools follows skill_categories, again without moving an earlier key.
	Tools []jsonTool `json:"tools"`
}

type jsonTool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Description string `json:"description"`
}

// jsonSkillCategory is one skill group. The list is in display order: by
// order where set, then file order.
type jsonSkillCategory struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order *int   `json:"order"`
}

type jsonExperience struct {
	ID             string  `json:"id"`
	Employer       string  `json:"employer"`
	Title          string  `json:"title"`
	Start          string  `json:"start"`
	End            *string `json:"end"`
	Summary        *string `json:"summary"`
	Location       *string `json:"location"`
	EmploymentType *string `json:"employment_type"`
}

type jsonEducation struct {
	ID          string  `json:"id"`
	Institution string  `json:"institution"`
	Degree      string  `json:"degree"`
	Start       string  `json:"start"`
	End         *string `json:"end"`
	Summary     *string `json:"summary"`
	Grade       *string `json:"grade"`
}

type jsonVolunteering struct {
	ID           string  `json:"id"`
	Organisation string  `json:"organisation"`
	Role         string  `json:"role"`
	Cause        *string `json:"cause"`
	Start        string  `json:"start"`
	End          *string `json:"end"`
	Summary      *string `json:"summary"`
}

type jsonAward struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Issuer  string  `json:"issuer"`
	Date    string  `json:"date"`
	Summary *string `json:"summary"`
}

type jsonExamResult struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Score   string  `json:"score"`
	Date    string  `json:"date"`
	Summary *string `json:"summary"`
}

type jsonCourse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Provider *string `json:"provider"`
	Date     *string `json:"date"`
}

type jsonPublication struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Publisher *string `json:"publisher"`
	Date      string  `json:"date"`
	URL       *string `json:"url"`
	Summary   *string `json:"summary"`
}

type jsonPatent struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Number  *string `json:"number"`
	Date    string  `json:"date"`
	URL     *string `json:"url"`
	Summary *string `json:"summary"`
}

type jsonOrganisation struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Role  *string `json:"role"`
	Start string  `json:"start"`
	End   *string `json:"end"`
}

type jsonEngagement struct {
	ID         string  `json:"id"`
	Experience string  `json:"experience"`
	Title      string  `json:"title"`
	Industry   *string `json:"industry"`
	Start      *string `json:"start"`
	End        *string `json:"end"`
	Summary    string  `json:"summary"`
}

type jsonCertificate struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Issuer     string  `json:"issuer"`
	Issued     string  `json:"issued"`
	ValidUntil *string `json:"valid_until"`
}

type jsonEvidence struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

type jsonSkill struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Level    string         `json:"level"`
	Evidence []jsonEvidence `json:"evidence"`
	Category string         `json:"category"`
}

type jsonProject struct {
	ID          string `json:"id"`
	GithubRepo  string `json:"github_repo"`
	Description string `json:"description"`
}

type jsonReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type jsonLanguage struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
}

type jsonAddress struct {
	Street *string `json:"street"`
	City   *string `json:"city"`
}

type jsonProfile struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type jsonContact struct {
	Email    string        `json:"email"`
	Address  *jsonAddress  `json:"address"`
	Profiles []jsonProfile `json:"profiles"`
}

// RenderJSON renders doc, the data file of language lang, as the public JSON
// document: two space indentation, a trailing newline, no timestamp and no
// map anywhere, so the same input always yields the same bytes.
//
// docs/concepts/cv.md's "No visibility" settles that the repository holds
// only publishable data, so nothing is filtered here: every entry in doc
// reaches the output. RenderJSON does not validate doc; the caller does,
// through Load or LoadPair, before it renders.
func RenderJSON(doc Document, lang string) ([]byte, error) {
	known := false
	for _, l := range Languages {
		known = known || l == lang
	}
	if !known {
		return nil, fmt.Errorf("cv: render json: unknown language %q", lang)
	}

	out := jsonDocument{
		SchemaVersion: SchemaVersion,
		Language:      lang,
		Experience:    make([]jsonExperience, 0, len(doc.Experience)),
		Education:     make([]jsonEducation, 0, len(doc.Education)),
		Certificates:  make([]jsonCertificate, 0, len(doc.Certificates)),
		Skills:        make([]jsonSkill, 0, len(doc.Skills)),
		Projects:      make([]jsonProject, 0, len(doc.Projects)),
		References:    make([]jsonReference, 0, len(doc.References)),
		Languages:     make([]jsonLanguage, 0, len(doc.Languages)),
		Contact: jsonContact{
			Email:    doc.Contact.Email,
			Profiles: make([]jsonProfile, 0, len(doc.Contact.Profiles)),
		},
		Volunteering:  make([]jsonVolunteering, 0, len(doc.Volunteering)),
		Awards:        make([]jsonAward, 0, len(doc.Awards)),
		ExamResults:   make([]jsonExamResult, 0, len(doc.ExamResults)),
		Courses:       make([]jsonCourse, 0, len(doc.Courses)),
		Publications:  make([]jsonPublication, 0, len(doc.Publications)),
		Patents:       make([]jsonPatent, 0, len(doc.Patents)),
		Organisations: make([]jsonOrganisation, 0, len(doc.Organisations)),
		Engagements:   make([]jsonEngagement, 0, len(doc.Engagements)),

		SkillCategories: make([]jsonSkillCategory, 0, len(doc.SkillCategories)),
		Tools:           make([]jsonTool, 0, len(doc.Tools)),
	}

	for _, e := range doc.Tools {
		out.Tools = append(out.Tools, jsonTool{ID: e.ID, Name: e.Name, Group: string(e.Group), Description: e.Description})
	}

	for _, e := range sortedCategories(doc.SkillCategories) {
		out.SkillCategories = append(out.SkillCategories, jsonSkillCategory(e))
	}

	for _, e := range doc.Experience {
		out.Experience = append(out.Experience, jsonExperience{
			ID: e.ID, Employer: e.Employer, Title: e.Title, Start: e.Start,
			End: optional(e.End), Summary: optional(e.Summary),
			Location: optional(e.Location), EmploymentType: optional(e.EmploymentType),
		})
	}
	for _, e := range doc.Education {
		out.Education = append(out.Education, jsonEducation{
			ID: e.ID, Institution: e.Institution, Degree: e.Degree, Start: e.Start,
			End: optional(e.End), Summary: optional(e.Summary), Grade: optional(e.Grade),
		})
	}
	for _, e := range doc.Volunteering {
		out.Volunteering = append(out.Volunteering, jsonVolunteering{
			ID: e.ID, Organisation: e.Organisation, Role: e.Role, Cause: optional(e.Cause),
			Start: e.Start, End: optional(e.End), Summary: optional(e.Summary),
		})
	}
	for _, e := range doc.Awards {
		out.Awards = append(out.Awards, jsonAward{
			ID: e.ID, Title: e.Title, Issuer: e.Issuer, Date: e.Date, Summary: optional(e.Summary),
		})
	}
	for _, e := range doc.ExamResults {
		out.ExamResults = append(out.ExamResults, jsonExamResult{
			ID: e.ID, Name: e.Name, Score: e.Score, Date: e.Date, Summary: optional(e.Summary),
		})
	}
	for _, e := range doc.Courses {
		out.Courses = append(out.Courses, jsonCourse{
			ID: e.ID, Name: e.Name, Provider: optional(e.Provider), Date: optional(e.Date),
		})
	}
	for _, e := range doc.Publications {
		out.Publications = append(out.Publications, jsonPublication{
			ID: e.ID, Title: e.Title, Publisher: optional(e.Publisher), Date: e.Date,
			URL: optional(e.URL), Summary: optional(e.Summary),
		})
	}
	for _, e := range doc.Patents {
		out.Patents = append(out.Patents, jsonPatent{
			ID: e.ID, Title: e.Title, Number: optional(e.Number), Date: e.Date,
			URL: optional(e.URL), Summary: optional(e.Summary),
		})
	}
	for _, e := range doc.Organisations {
		out.Organisations = append(out.Organisations, jsonOrganisation{
			ID: e.ID, Name: e.Name, Role: optional(e.Role), Start: e.Start, End: optional(e.End),
		})
	}
	for _, e := range doc.Engagements {
		out.Engagements = append(out.Engagements, jsonEngagement{
			ID: e.ID, Experience: e.Experience, Title: e.Title, Industry: optional(e.Industry),
			Start: optional(e.Start), End: optional(e.End), Summary: e.Summary,
		})
	}
	for _, e := range doc.Certificates {
		out.Certificates = append(out.Certificates, jsonCertificate{
			ID: e.ID, Name: e.Name, Issuer: e.Issuer, Issued: e.Issued, ValidUntil: optional(e.ValidUntil),
		})
	}
	for _, e := range doc.Skills {
		evidence := make([]jsonEvidence, 0, len(e.Evidence))
		for _, ev := range e.Evidence {
			evidence = append(evidence, jsonEvidence{Type: string(ev.Type), Ref: ev.Ref})
		}
		out.Skills = append(out.Skills, jsonSkill{
			ID: e.ID, Name: e.Name, Level: string(e.Level), Evidence: evidence,
			Category: e.Category,
		})
	}
	for _, e := range doc.Projects {
		out.Projects = append(out.Projects, jsonProject(e))
	}
	for _, e := range doc.References {
		out.References = append(out.References, jsonReference(e))
	}
	for _, e := range doc.Languages {
		out.Languages = append(out.Languages, jsonLanguage{ID: e.ID, Name: e.Name, Level: string(e.Level)})
	}
	if a := doc.Contact.Address; a != nil {
		out.Contact.Address = &jsonAddress{Street: optional(a.Street), City: optional(a.City)}
	}
	for _, p := range doc.Contact.Profiles {
		out.Contact.Profiles = append(out.Contact.Profiles, jsonProfile(p))
	}

	trimStrings(reflect.ValueOf(&out).Elem())

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("cv: render json: %w", err)
	}
	return buf.Bytes(), nil
}

// sortedCategories returns a copy of entries in display order: entries with
// an order first, ascending, then the rest, each group in file order.
func sortedCategories(entries []SkillCategoryEntry) []SkillCategoryEntry {
	out := slices.Clone(entries)
	slices.SortStableFunc(out, func(a, b SkillCategoryEntry) int {
		switch {
		case a.Order != nil && b.Order != nil:
			return cmp.Compare(*a.Order, *b.Order)
		case a.Order != nil:
			return -1
		case b.Order != nil:
			return 1
		}
		return 0
	})
	return out
}

// trimStrings removes leading and trailing whitespace from every string in
// the output value v, at any depth, and sets an optional (*string) value that
// ends up empty to nil, so it encodes as null. YAML's folded `>` style keeps a
// trailing newline, which is formatting of the data file, not content. Walking
// the output, not the model, means a field added later is covered without
// anyone remembering to list it. Inner whitespace stays.
func trimStrings(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(strings.TrimSpace(v.String()))
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		if v.Elem().Kind() == reflect.String {
			trimmed := strings.TrimSpace(v.Elem().String())
			if trimmed == "" {
				v.Set(reflect.Zero(v.Type()))
				return
			}
			s := trimmed
			v.Set(reflect.ValueOf(&s))
			return
		}
		trimStrings(v.Elem())
	case reflect.Struct:
		for i := range v.NumField() {
			trimStrings(v.Field(i))
		}
	case reflect.Slice:
		for i := range v.Len() {
			trimStrings(v.Index(i))
		}
	}
}

// optional maps an unset optional string to nil, which encodes as null.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
