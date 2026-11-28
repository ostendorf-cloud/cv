package client

// Document is the JSON document cvd serves for one language. It mirrors the
// public contract of internal/cv's RenderJSON: field names follow the keys,
// an optional scalar is a pointer that is nil for null, and a list is empty,
// never nil, once decoded from a document that carries the key. A drift test
// decodes what the real renderer writes into these types and encodes it
// again, so a key added to or removed from the contract fails the build.
//
// Unknown keys are ignored when decoding, so a consumer keeps working when
// the server adds a key without raising SchemaVersion.
type Document struct {
	SchemaVersion int              `json:"schema_version"`
	Language      string           `json:"language"`
	Experience    []Experience     `json:"experience"`
	Education     []Education      `json:"education"`
	Certificates  []Certificate    `json:"certificates"`
	Skills        []Skill          `json:"skills"`
	Projects      []Project        `json:"projects"`
	References    []Reference      `json:"references"`
	Languages     []SpokenLanguage `json:"languages"`
	Contact       Contact          `json:"contact"`

	Volunteering  []Volunteering `json:"volunteering"`
	Awards        []Award        `json:"awards"`
	ExamResults   []ExamResult   `json:"exam_results"`
	Courses       []Course       `json:"courses"`
	Publications  []Publication  `json:"publications"`
	Patents       []Patent       `json:"patents"`
	Organisations []Organisation `json:"organisations"`
	Engagements   []Engagement   `json:"engagements"`

	// SkillCategories is in display order.
	SkillCategories []SkillCategory `json:"skill_categories"`
	Tools           []Tool          `json:"tools"`
}

// Tool is one tool of the toolbox. Group is one of the server's enumerated
// groups, kept as a string so a new group needs no release of this package.
type Tool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Description string `json:"description"`
}

// SkillCategory is one skill group. Order is nil for a category without one.
type SkillCategory struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order *int   `json:"order"`
}

// Experience is one position.
type Experience struct {
	ID             string  `json:"id"`
	Employer       string  `json:"employer"`
	Title          string  `json:"title"`
	Start          string  `json:"start"`
	End            *string `json:"end"`
	Summary        *string `json:"summary"`
	Location       *string `json:"location"`
	EmploymentType *string `json:"employment_type"`
}

// Education is one course of study.
type Education struct {
	ID          string  `json:"id"`
	Institution string  `json:"institution"`
	Degree      string  `json:"degree"`
	Start       string  `json:"start"`
	End         *string `json:"end"`
	Summary     *string `json:"summary"`
	Grade       *string `json:"grade"`
}

// Volunteering is one voluntary engagement.
type Volunteering struct {
	ID           string  `json:"id"`
	Organisation string  `json:"organisation"`
	Role         string  `json:"role"`
	Cause        *string `json:"cause"`
	Start        string  `json:"start"`
	End          *string `json:"end"`
	Summary      *string `json:"summary"`
}

// Award is one award.
type Award struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Issuer  string  `json:"issuer"`
	Date    string  `json:"date"`
	Summary *string `json:"summary"`
}

// ExamResult is one exam and its score.
type ExamResult struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Score   string  `json:"score"`
	Date    string  `json:"date"`
	Summary *string `json:"summary"`
}

// Course is one course.
type Course struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Provider *string `json:"provider"`
	Date     *string `json:"date"`
}

// Publication is one publication.
type Publication struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Publisher *string `json:"publisher"`
	Date      string  `json:"date"`
	URL       *string `json:"url"`
	Summary   *string `json:"summary"`
}

// Patent is one patent.
type Patent struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Number  *string `json:"number"`
	Date    string  `json:"date"`
	URL     *string `json:"url"`
	Summary *string `json:"summary"`
}

// Organisation is one membership.
type Organisation struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Role  *string `json:"role"`
	Start string  `json:"start"`
	End   *string `json:"end"`
}

// Engagement is one project of a position that has no repository. The
// server anonymises it.
type Engagement struct {
	ID         string  `json:"id"`
	Experience string  `json:"experience"`
	Title      string  `json:"title"`
	Industry   *string `json:"industry"`
	Start      *string `json:"start"`
	End        *string `json:"end"`
	Summary    string  `json:"summary"`
}

// Certificate is one certificate.
type Certificate struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Issuer     string  `json:"issuer"`
	Issued     string  `json:"issued"`
	ValidUntil *string `json:"valid_until"`
}

// Evidence points from a skill to the entry that shows it. Type is one of
// the server's evidence types and Ref the id of an entry of that type.
type Evidence struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

// Skill is one skill with its level and the category it belongs to.
type Skill struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Level    string     `json:"level"`
	Evidence []Evidence `json:"evidence"`
	Category string     `json:"category"`
}

// Project is one project with a public repository. Languages and
// LatestRelease are the GitHub facts cvd merges in. Both are absent, so nil,
// for a project without them: before the first lookup ended, after a lookup
// that never succeeded, or for a repository that reports no language.
type Project struct {
	ID            string            `json:"id"`
	GithubRepo    string            `json:"github_repo"`
	Description   string            `json:"description"`
	Languages     []ProjectLanguage `json:"languages,omitempty"`
	LatestRelease *ProjectRelease   `json:"latest_release,omitempty"`
}

// ProjectLanguage is one language of a repository and its share in bytes,
// largest first.
type ProjectLanguage struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// ProjectRelease is the latest published release of a repository.
// PublishedAt is an RFC 3339 timestamp as GitHub reports it.
type ProjectRelease struct {
	Tag         string  `json:"tag"`
	Name        *string `json:"name"`
	URL         *string `json:"url"`
	PublishedAt *string `json:"published_at"`
}

// Reference is one reference person.
type Reference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// SpokenLanguage is a language the owner speaks, with a level. It is not the
// language of the document, see Document.Language.
type SpokenLanguage struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
}

// Address is the postal address. Either part may be nil.
type Address struct {
	Street *string `json:"street"`
	City   *string `json:"city"`
}

// Profile is one profile on a platform.
type Profile struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

// Contact is how to reach the owner. Address is nil when none is published.
type Contact struct {
	Email    string    `json:"email"`
	Address  *Address  `json:"address"`
	Profiles []Profile `json:"profiles"`
}
