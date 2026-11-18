// Package cv holds the typed data model behind Simon Ostendorf's curriculum
// vitae and the validation every loaded data file goes through before any
// renderer or service reads it.
//
// The model mirrors docs/concepts/cv.md's "Four areas, eight top level
// sections": experience, education, certificates, skills, projects,
// references, languages and contact, plus seven optional sections that
// hold what a LinkedIn profile holds: volunteering, awards, exam_results,
// courses, publications, patents and organisations. Every entry in every section carries a
// required, hand assigned id, never derived from its own text, which is what
// makes an entry the same entry across data/cv.de.yaml and data/cv.en.yaml.
// See Load, LoadPair and Validate for the checks each file and each pair of
// files goes through.
package cv

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

	// The sections below are optional lists. They sit after Contact, in the
	// order the JSON document carries them.
	Volunteering  []VolunteeringEntry `yaml:"volunteering"`
	Awards        []AwardEntry        `yaml:"awards"`
	ExamResults   []ExamResultEntry   `yaml:"exam_results"`
	Courses       []CourseEntry       `yaml:"courses"`
	Publications  []PublicationEntry  `yaml:"publications"`
	Patents       []PatentEntry       `yaml:"patents"`
	Organisations []OrganisationEntry `yaml:"organisations"`
}

// ExperienceEntry is one career entry. Required: ID, Employer, Title, Start.
// End and Summary are optional, matching docs/concepts/cv.md's own example:
// an ongoing role has no End. Location and EmploymentType are optional free
// text, such as "Berlin" and "Full-time".
type ExperienceEntry struct {
	ID             string `yaml:"id"`
	Employer       string `yaml:"employer"`
	Title          string `yaml:"title"`
	Start          string `yaml:"start"`
	End            string `yaml:"end,omitempty"`
	Summary        string `yaml:"summary,omitempty"`
	Location       string `yaml:"location,omitempty"`
	EmploymentType string `yaml:"employment_type,omitempty"`
}

func (e ExperienceEntry) identity() string { return e.ID }

// EducationEntry is one degree or programme. Required: ID, Institution,
// Degree, Start. End and Summary are optional, for the same reason as
// ExperienceEntry's: a programme still in progress has no End. Grade is
// optional free text, such as "1,6".
type EducationEntry struct {
	ID          string `yaml:"id"`
	Institution string `yaml:"institution"`
	Degree      string `yaml:"degree"`
	Start       string `yaml:"start"`
	End         string `yaml:"end,omitempty"`
	Summary     string `yaml:"summary,omitempty"`
	Grade       string `yaml:"grade,omitempty"`
}

func (e EducationEntry) identity() string { return e.ID }

// CertificateEntry is one issued credential. Required: ID, Name, Issuer,
// Issued, ValidUntil. The validity window is why this is its own section
// rather than folded into Skills' evidence.
type CertificateEntry struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	Issuer     string `yaml:"issuer"`
	Issued     string `yaml:"issued"`
	ValidUntil string `yaml:"valid_until"`
}

func (e CertificateEntry) identity() string { return e.ID }

// Evidence links a skill back to the project, experience or certificate id
// that backs it up.
type Evidence struct {
	Type EvidenceType `yaml:"type"`
	Ref  string       `yaml:"ref"`
}

// SkillEntry is one claimed skill. Required: ID, Name, Level. Evidence is
// optional; when present, every entry's Ref is checked against the ids that
// actually exist in the matching section.
type SkillEntry struct {
	ID       string     `yaml:"id"`
	Name     string     `yaml:"name"`
	Level    Level      `yaml:"level"`
	Evidence []Evidence `yaml:"evidence,omitempty"`
}

func (e SkillEntry) identity() string { return e.ID }

// ProjectEntry is a hand written project description plus the GitHub
// repository it enriches from. Required: ID, GithubRepo, Description.
// docs/concepts/cv.md is explicit that languages, stars and latest_release
// are never stored here; they are merged in at render time, so this type
// carries no field for any of the three, and the strict YAML decoder in
// Parse rejects a data file that tries to set one anyway.
type ProjectEntry struct {
	ID          string `yaml:"id"`
	GithubRepo  string `yaml:"github_repo"`
	Description string `yaml:"description"`
}

func (e ProjectEntry) identity() string { return e.ID }

// ReferenceEntry is one person who can vouch. Required: ID, Name, Role.
type ReferenceEntry struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

func (e ReferenceEntry) identity() string { return e.ID }

// LanguageEntry is one spoken language. Required: ID, Name, Level. Level
// reuses the same four-step scale Skills uses; see Level's own doc comment
// for why.
type LanguageEntry struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Level Level  `yaml:"level"`
}

func (e LanguageEntry) identity() string { return e.ID }

// Address is the contact section's optional postal address. Street and City
// are both optional; nothing in this package, and no sink that reads its
// output, filters an address out, so a committed address is as public as
// every other field here.
type Address struct {
	Street string `yaml:"street,omitempty"`
	City   string `yaml:"city,omitempty"`
}

// Profile is one entry in Contact's Profiles list: a link to a social or
// professional profile. Required: ID, Platform, URL.
type Profile struct {
	ID       string `yaml:"id"`
	Platform string `yaml:"platform"`
	URL      string `yaml:"url"`
}

func (p Profile) identity() string { return p.ID }

// Contact is the flat set of contact fields plus the Profiles list.
// Required: Email. Address is optional; Profiles may be empty, but every
// entry in it is required to carry its own three fields, checked the same
// way every other section's entries are.
type Contact struct {
	Email    string    `yaml:"email"`
	Address  *Address  `yaml:"address,omitempty"`
	Profiles []Profile `yaml:"profiles,omitempty"`
}

// VolunteeringEntry is one volunteer role. Required: ID, Organisation, Role,
// Start. Cause, End and Summary are optional. Dates are "YYYY-MM".
type VolunteeringEntry struct {
	ID           string `yaml:"id"`
	Organisation string `yaml:"organisation"`
	Role         string `yaml:"role"`
	Cause        string `yaml:"cause,omitempty"`
	Start        string `yaml:"start"`
	End          string `yaml:"end,omitempty"`
	Summary      string `yaml:"summary,omitempty"`
}

func (e VolunteeringEntry) identity() string { return e.ID }

// AwardEntry is one honor or award. Required: ID, Title, Issuer, Date.
// Summary is optional. Date is "YYYY-MM".
type AwardEntry struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
	Issuer  string `yaml:"issuer"`
	Date    string `yaml:"date"`
	Summary string `yaml:"summary,omitempty"`
}

func (e AwardEntry) identity() string { return e.ID }

// ExamResultEntry is one test score, such as a thesis grade. Required: ID,
// Name, Score, Date. Score is free text, not a number, because grading
// schemes differ. Summary is optional. Date is "YYYY-MM".
type ExamResultEntry struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Score   string `yaml:"score"`
	Date    string `yaml:"date"`
	Summary string `yaml:"summary,omitempty"`
}

func (e ExamResultEntry) identity() string { return e.ID }

// CourseEntry is one course taken. Required: ID, Name. Provider and Date are
// optional. Date is "YYYY-MM".
type CourseEntry struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Provider string `yaml:"provider,omitempty"`
	Date     string `yaml:"date,omitempty"`
}

func (e CourseEntry) identity() string { return e.ID }

// PublicationEntry is one publication. Required: ID, Title, Date. Publisher,
// URL and Summary are optional. Date is "YYYY-MM".
type PublicationEntry struct {
	ID        string `yaml:"id"`
	Title     string `yaml:"title"`
	Publisher string `yaml:"publisher,omitempty"`
	Date      string `yaml:"date"`
	URL       string `yaml:"url,omitempty"`
	Summary   string `yaml:"summary,omitempty"`
}

func (e PublicationEntry) identity() string { return e.ID }

// PatentEntry is one patent. Required: ID, Title, Date. Number, URL and
// Summary are optional. Date is "YYYY-MM".
type PatentEntry struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
	Number  string `yaml:"number,omitempty"`
	Date    string `yaml:"date"`
	URL     string `yaml:"url,omitempty"`
	Summary string `yaml:"summary,omitempty"`
}

func (e PatentEntry) identity() string { return e.ID }

// OrganisationEntry is one membership. Required: ID, Name, Start. Role and
// End are optional. Dates are "YYYY-MM".
type OrganisationEntry struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Role  string `yaml:"role,omitempty"`
	Start string `yaml:"start"`
	End   string `yaml:"end,omitempty"`
}

func (e OrganisationEntry) identity() string { return e.ID }

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
	"volunteering",
	"awards",
	"exam_results",
	"courses",
	"publications",
	"patents",
	"organisations",
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

		"volunteering":  ids(doc.Volunteering),
		"awards":        ids(doc.Awards),
		"exam_results":  ids(doc.ExamResults),
		"courses":       ids(doc.Courses),
		"publications":  ids(doc.Publications),
		"patents":       ids(doc.Patents),
		"organisations": ids(doc.Organisations),
	}
}
