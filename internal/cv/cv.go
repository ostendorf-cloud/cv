// Package cv holds the typed data model behind Simon Ostendorf's curriculum
// vitae and the validation every loaded data file goes through before any
// renderer or service reads it.
//
// The model mirrors docs/concepts/cv.md's "Four areas, eight top level
// sections": experience, education, certificates, skills, projects,
// references, languages and contact. Every entry in every section carries a
// required, hand assigned id, never derived from its own text, which is what
// makes an entry the same entry across data/cv.de.yaml and data/cv.en.yaml.
// See Load, LoadPair and Validate for the checks each file and each pair of
// files goes through.
package cv

// Visibility decides which sinks an entry may appear in. The concept
// document defines exactly two levels and settles that filtering happens
// once, in the renderer that reads this package's output.
type Visibility string

// The two visibility levels docs/concepts/cv.md defines. Public may appear
// in every sink; cv-only is for the PDF alone.
const (
	VisibilityPublic Visibility = "public"
	VisibilityCVOnly Visibility = "cv-only"
)

// visibilities is every value Visibility accepts, used to validate a loaded
// value and to report an unknown one by name.
var visibilities = []Visibility{VisibilityPublic, VisibilityCVOnly}

// Level is a proficiency level. docs/concepts/cv.md defines exactly one
// four-step scale, under the skills section, and does not define a separate
// scale for languages. This package reuses the same scale for skills and
// languages rather than inventing a second one the concept document never
// asked for; a future language specific scale, such as CEFR, is a schema
// change for whoever owns that decision, not something to anticipate here.
type Level string

// The four levels docs/concepts/cv.md defines.
const (
	LevelFamiliar   Level = "familiar"
	LevelWorking    Level = "working"
	LevelProficient Level = "proficient"
	LevelExpert     Level = "expert"
)

// levels is every value Level accepts.
var levels = []Level{LevelFamiliar, LevelWorking, LevelProficient, LevelExpert}

// EvidenceType is what kind of entry a piece of skill evidence points at.
type EvidenceType string

// The three evidence types docs/concepts/cv.md names: a skill's evidence
// links back to a project, an experience or a certificate id.
const (
	EvidenceTypeProject     EvidenceType = "project"
	EvidenceTypeExperience  EvidenceType = "experience"
	EvidenceTypeCertificate EvidenceType = "certificate"
)

// Document is one data file, data/cv.de.yaml or data/cv.en.yaml, decoded
// into the eight top level sections docs/concepts/cv.md defines. Both files
// decode into the same Go type; nothing here merges the two languages, that
// stays the renderer's job once one exists.
type Document struct {
	Experience   []ExperienceEntry  `yaml:"experience"`
	Education    []EducationEntry   `yaml:"education"`
	Certificates []CertificateEntry `yaml:"certificates"`
	Skills       []SkillEntry       `yaml:"skills"`
	Projects     []ProjectEntry     `yaml:"projects"`
	References   []ReferenceEntry   `yaml:"references"`
	Languages    []LanguageEntry    `yaml:"languages"`
	Contact      Contact            `yaml:"contact"`
}

// ExperienceEntry is one career entry. Required: ID, Employer, Title, Start,
// Visibility. End and Summary are optional, matching
// docs/concepts/cv.md's own example: an ongoing role has no End.
type ExperienceEntry struct {
	ID         string     `yaml:"id"`
	Employer   string     `yaml:"employer"`
	Title      string     `yaml:"title"`
	Start      string     `yaml:"start"`
	End        string     `yaml:"end,omitempty"`
	Visibility Visibility `yaml:"visibility"`
	Summary    string     `yaml:"summary,omitempty"`
}

func (e ExperienceEntry) identity() string { return e.ID }

// EducationEntry is one degree or programme. Required: ID, Institution,
// Degree, Start, Visibility. End and Summary are optional, for the same
// reason as ExperienceEntry's: a programme still in progress has no End.
type EducationEntry struct {
	ID          string     `yaml:"id"`
	Institution string     `yaml:"institution"`
	Degree      string     `yaml:"degree"`
	Start       string     `yaml:"start"`
	End         string     `yaml:"end,omitempty"`
	Visibility  Visibility `yaml:"visibility"`
	Summary     string     `yaml:"summary,omitempty"`
}

func (e EducationEntry) identity() string { return e.ID }

// CertificateEntry is one issued credential. Required: ID, Name, Issuer,
// Issued, ValidUntil, Visibility. The validity window is why this is its own
// section rather than folded into Skills' evidence.
type CertificateEntry struct {
	ID         string     `yaml:"id"`
	Name       string     `yaml:"name"`
	Issuer     string     `yaml:"issuer"`
	Issued     string     `yaml:"issued"`
	ValidUntil string     `yaml:"valid_until"`
	Visibility Visibility `yaml:"visibility"`
}

func (e CertificateEntry) identity() string { return e.ID }

// Evidence links a skill back to the project, experience or certificate id
// that backs it up.
type Evidence struct {
	Type EvidenceType `yaml:"type"`
	Ref  string       `yaml:"ref"`
}

// SkillEntry is one claimed skill. Required: ID, Name, Level, Visibility.
// Evidence is optional; when present, every entry's Ref is checked against
// the ids that actually exist in the matching section.
type SkillEntry struct {
	ID         string     `yaml:"id"`
	Name       string     `yaml:"name"`
	Level      Level      `yaml:"level"`
	Visibility Visibility `yaml:"visibility"`
	Evidence   []Evidence `yaml:"evidence,omitempty"`
}

func (e SkillEntry) identity() string { return e.ID }

// ProjectEntry is a hand written project description plus the GitHub
// repository it enriches from. Required: ID, GithubRepo, Description,
// Visibility. docs/concepts/cv.md is explicit that languages, stars and
// latest_release are never stored here; they are merged in at render time,
// so this type carries no field for any of the three, and the strict YAML
// decoder in Parse rejects a data file that tries to set one anyway.
type ProjectEntry struct {
	ID          string     `yaml:"id"`
	GithubRepo  string     `yaml:"github_repo"`
	Description string     `yaml:"description"`
	Visibility  Visibility `yaml:"visibility"`
}

func (e ProjectEntry) identity() string { return e.ID }

// ReferenceEntry is one person who can vouch, almost always cv-only.
// Required: ID, Name, Role, Visibility.
type ReferenceEntry struct {
	ID         string     `yaml:"id"`
	Name       string     `yaml:"name"`
	Role       string     `yaml:"role"`
	Visibility Visibility `yaml:"visibility"`
}

func (e ReferenceEntry) identity() string { return e.ID }

// LanguageEntry is one spoken language. Required: ID, Name, Level,
// Visibility. Level reuses the same four-step scale Skills uses; see Level's
// own doc comment for why.
type LanguageEntry struct {
	ID         string     `yaml:"id"`
	Name       string     `yaml:"name"`
	Level      Level      `yaml:"level"`
	Visibility Visibility `yaml:"visibility"`
}

func (e LanguageEntry) identity() string { return e.ID }

// Address is the contact section's optional postal address, almost always
// cv-only: docs/concepts/cv.md names it directly as the example of content
// that belongs on the PDF and nowhere else.
type Address struct {
	Visibility Visibility `yaml:"visibility"`
	Street     string     `yaml:"street,omitempty"`
	City       string     `yaml:"city,omitempty"`
}

// Profile is one entry in Contact's Profiles list: a link to a social or
// professional profile. Required: ID, Platform, URL, Visibility.
type Profile struct {
	ID         string     `yaml:"id"`
	Platform   string     `yaml:"platform"`
	URL        string     `yaml:"url"`
	Visibility Visibility `yaml:"visibility"`
}

func (p Profile) identity() string { return p.ID }

// Contact is the flat set of contact fields plus the Profiles list.
// Required: Email. Address is optional; Profiles may be empty, but every
// entry in it is required to carry its own four fields, checked the same
// way every other section's entries are.
type Contact struct {
	Email    string    `yaml:"email"`
	Address  *Address  `yaml:"address,omitempty"`
	Profiles []Profile `yaml:"profiles,omitempty"`
}

// sectionNames is every top level section docs/concepts/cv.md defines, in
// the order it defines them, used wherever a section is iterated by name:
// the duplicate id check, the cross reference check and the two-file drift
// check.
var sectionNames = []string{
	"experience",
	"education",
	"certificates",
	"skills",
	"projects",
	"references",
	"languages",
	"contact",
}

// identified is implemented by every entry type, so one generic helper can
// collect a section's ids regardless of what else the entry carries.
type identified interface {
	identity() string
}

// ids returns entries's own ids, in file order, duplicates included: the
// caller decides what a duplicate means.
func ids[T identified](entries []T) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.identity()
	}
	return out
}

// sectionIDs maps every section name to the ids its entries carry, in file
// order. Contact has no ids of its own; it maps to its Profiles' ids, the
// only id-bearing list the section holds.
func sectionIDs(doc Document) map[string][]string {
	return map[string][]string{
		"experience":   ids(doc.Experience),
		"education":    ids(doc.Education),
		"certificates": ids(doc.Certificates),
		"skills":       ids(doc.Skills),
		"projects":     ids(doc.Projects),
		"references":   ids(doc.References),
		"languages":    ids(doc.Languages),
		"contact":      ids(doc.Contact.Profiles),
	}
}
