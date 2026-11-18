package cv

import (
	"bytes"
	"encoding/json"
	"fmt"
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
}

type jsonExperience struct {
	ID       string  `json:"id"`
	Employer string  `json:"employer"`
	Title    string  `json:"title"`
	Start    string  `json:"start"`
	End      *string `json:"end"`
	Summary  *string `json:"summary"`
}

type jsonEducation struct {
	ID          string  `json:"id"`
	Institution string  `json:"institution"`
	Degree      string  `json:"degree"`
	Start       string  `json:"start"`
	End         *string `json:"end"`
	Summary     *string `json:"summary"`
}

type jsonCertificate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Issuer     string `json:"issuer"`
	Issued     string `json:"issued"`
	ValidUntil string `json:"valid_until"`
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
	}

	for _, e := range doc.Experience {
		out.Experience = append(out.Experience, jsonExperience{
			ID: e.ID, Employer: e.Employer, Title: e.Title, Start: e.Start,
			End: optional(e.End), Summary: optional(e.Summary),
		})
	}
	for _, e := range doc.Education {
		out.Education = append(out.Education, jsonEducation{
			ID: e.ID, Institution: e.Institution, Degree: e.Degree, Start: e.Start,
			End: optional(e.End), Summary: optional(e.Summary),
		})
	}
	for _, e := range doc.Certificates {
		out.Certificates = append(out.Certificates, jsonCertificate(e))
	}
	for _, e := range doc.Skills {
		evidence := make([]jsonEvidence, 0, len(e.Evidence))
		for _, ev := range e.Evidence {
			evidence = append(evidence, jsonEvidence{Type: string(ev.Type), Ref: ev.Ref})
		}
		out.Skills = append(out.Skills, jsonSkill{
			ID: e.ID, Name: e.Name, Level: string(e.Level), Evidence: evidence,
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

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("cv: render json: %w", err)
	}
	return buf.Bytes(), nil
}

// optional maps an unset optional string to nil, which encodes as null.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
