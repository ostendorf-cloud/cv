package cv

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// validator collects every problem found while checking one Document,
// instead of stopping at the first, the same way cvdconfig.Load aggregates
// cvd's own configuration errors.
type validator struct {
	source   string
	problems []error
}

// fail records one problem, scoped to the entry or field ref names.
func (v *validator) fail(ref, format string, args ...any) {
	v.problems = append(v.problems, fmt.Errorf("cv: %s: %s: %s", v.source, ref, fmt.Sprintf(format, args...)))
}

// err is every problem found, joined, or nil.
func (v *validator) err() error {
	return errors.Join(v.problems...)
}

// Validate checks doc against every rule docs/concepts/cv.md states: the
// required fields per section, the four skill levels, a skill's category
// against the skill categories that exist, a duplicate id within
// a section, and a skill's evidence against the project, experience and
// certificate ids that actually exist. source names the file doc came from,
// for the error messages only.
//
// Every problem Validate finds is reported together in the returned error;
// it never stops at the first one.
func Validate(doc Document, source string) error {
	v := &validator{source: source}
	categoryIDs := toSet(ids(doc.SkillCategories))

	for i, e := range doc.Experience {
		ref := entryRef("experience", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "employer", e.Employer)
		v.required(ref, "title", e.Title)
		v.required(ref, "start", e.Start)
	}
	for i, e := range doc.Education {
		ref := entryRef("education", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "institution", e.Institution)
		v.required(ref, "degree", e.Degree)
		v.required(ref, "start", e.Start)
	}
	for i, e := range doc.Certificates {
		ref := entryRef("certificates", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.required(ref, "issuer", e.Issuer)
		v.requiredDate(ref, "issued", e.Issued)
		v.date(ref, "valid_until", e.ValidUntil)
		v.notBefore(ref, "valid_until", e.ValidUntil, "issued", e.Issued)
	}
	for i, e := range doc.Skills {
		ref := entryRef("skills", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.level(ref, e.Level)
		v.required(ref, "category", e.Category)
		if strings.TrimSpace(e.Category) != "" && !categoryIDs[e.Category] {
			v.fail(ref, "category: no skill category with id %q exists", e.Category)
		}
	}
	for i, e := range doc.SkillCategories {
		ref := entryRef("skill_categories", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		if e.Order != nil && *e.Order < 1 {
			v.fail(ref, "order: %d is not a positive number", *e.Order)
		}
	}
	for i, e := range doc.Projects {
		ref := entryRef("projects", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "github_repo", e.GithubRepo)
		v.required(ref, "description", e.Description)
	}
	for i, e := range doc.References {
		ref := entryRef("references", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.required(ref, "role", e.Role)
	}
	for i, e := range doc.Languages {
		ref := entryRef("languages", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.level(ref, e.Level)
	}

	v.required("contact", "email", doc.Contact.Email)
	for i, p := range doc.Contact.Profiles {
		ref := entryRef("contact.profiles", i, p.ID)
		v.required(ref, "id", p.ID)
		v.required(ref, "platform", p.Platform)
		v.required(ref, "url", p.URL)
	}

	v.volunteering(doc.Volunteering)
	v.awards(doc.Awards)
	v.examResults(doc.ExamResults)
	v.courses(doc.Courses)
	v.publications(doc.Publications)
	v.patents(doc.Patents)
	v.organisations(doc.Organisations)
	v.engagements(doc.Engagements, toSet(ids(doc.Experience)))
	v.tools(doc.Tools)

	for _, section := range sectionNames {
		for _, id := range duplicates(sectionIDs(doc)[section]) {
			v.fail(section, "duplicate id %q", id)
		}
	}

	v.skillEvidence(doc)

	return v.err()
}

// required fails ref when value is empty or only whitespace: the JSON
// renderer trims every text field, so a blank value would render as "".
func (v *validator) required(ref, field, value string) {
	if strings.TrimSpace(value) == "" {
		v.fail(ref, "%s: is required", field)
	}
}

// monthPattern is the one date format the optional sections accept: a year
// and a month, "YYYY-MM", month 01 to 12.
var monthPattern = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)

// date fails ref when value is set but is not a "YYYY-MM" month. An empty
// value passes: whether it is required is required's decision.
func (v *validator) date(ref, field, value string) {
	value = strings.TrimSpace(value)
	if value != "" && !monthPattern.MatchString(value) {
		v.fail(ref, "%s: %q is not a YYYY-MM date", field, value)
	}
}

// notBefore fails ref when both values are valid "YYYY-MM" months and later
// is earlier than earlier. A missing or malformed value is reported by
// required and date, not here. "YYYY-MM" strings compare in date order.
func (v *validator) notBefore(ref, laterField, later, earlierField, earlier string) {
	later, earlier = strings.TrimSpace(later), strings.TrimSpace(earlier)
	if !monthPattern.MatchString(later) || !monthPattern.MatchString(earlier) {
		return
	}
	if later < earlier {
		v.fail(ref, "%s: %q is before %s %q", laterField, later, earlierField, earlier)
	}
}

// requiredDate is required plus date.
func (v *validator) requiredDate(ref, field, value string) {
	v.required(ref, field, value)
	v.date(ref, field, value)
}

func (v *validator) volunteering(entries []VolunteeringEntry) {
	for i, e := range entries {
		ref := entryRef("volunteering", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "organisation", e.Organisation)
		v.required(ref, "role", e.Role)
		v.requiredDate(ref, "start", e.Start)
		v.date(ref, "end", e.End)
		v.notBefore(ref, "end", e.End, "start", e.Start)
	}
}

func (v *validator) awards(entries []AwardEntry) {
	for i, e := range entries {
		ref := entryRef("awards", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "title", e.Title)
		v.required(ref, "issuer", e.Issuer)
		v.requiredDate(ref, "date", e.Date)
	}
}

func (v *validator) examResults(entries []ExamResultEntry) {
	for i, e := range entries {
		ref := entryRef("exam_results", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.required(ref, "score", e.Score)
		v.requiredDate(ref, "date", e.Date)
	}
}

func (v *validator) courses(entries []CourseEntry) {
	for i, e := range entries {
		ref := entryRef("courses", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.date(ref, "date", e.Date)
	}
}

func (v *validator) publications(entries []PublicationEntry) {
	for i, e := range entries {
		ref := entryRef("publications", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "title", e.Title)
		v.requiredDate(ref, "date", e.Date)
	}
}

func (v *validator) patents(entries []PatentEntry) {
	for i, e := range entries {
		ref := entryRef("patents", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "title", e.Title)
		v.requiredDate(ref, "date", e.Date)
	}
}

func (v *validator) organisations(entries []OrganisationEntry) {
	for i, e := range entries {
		ref := entryRef("organisations", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.requiredDate(ref, "start", e.Start)
		v.date(ref, "end", e.End)
		v.notBefore(ref, "end", e.End, "start", e.Start)
	}
}

// yearOrMonthPattern is the date format of an engagement: "YYYY-MM" or "YYYY".
var yearOrMonthPattern = regexp.MustCompile(`^[0-9]{4}(-(0[1-9]|1[0-2]))?$`)

// yearOrMonth fails ref when value is set but is neither "YYYY-MM" nor "YYYY".
func (v *validator) yearOrMonth(ref, field, value string) {
	value = strings.TrimSpace(value)
	if value != "" && !yearOrMonthPattern.MatchString(value) {
		v.fail(ref, "%s: %q is not a YYYY-MM or YYYY date", field, value)
	}
}

// engagementEndNotBeforeStart fails ref when end is clearly before start. A
// bare year spans its whole year: as a start it means January, as an end it
// means December, so "2020-06" to "2020" is in order and "2020" to "2019-12"
// is not. A missing or malformed value is reported by yearOrMonth, not here.
func (v *validator) engagementEndNotBeforeStart(ref, start, end string) {
	start, end = strings.TrimSpace(start), strings.TrimSpace(end)
	if !yearOrMonthPattern.MatchString(start) || !yearOrMonthPattern.MatchString(end) {
		return
	}
	earliest, latest := start, end
	if len(earliest) == 4 {
		earliest += "-01"
	}
	if len(latest) == 4 {
		latest += "-12"
	}
	if latest < earliest {
		v.fail(ref, "end: %q is before start %q", end, start)
	}
}

func (v *validator) engagements(entries []EngagementEntry, experienceIDs map[string]bool) {
	for i, e := range entries {
		ref := entryRef("engagements", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "experience", e.Experience)
		v.required(ref, "title", e.Title)
		v.required(ref, "summary", e.Summary)
		if strings.TrimSpace(e.Experience) != "" && !experienceIDs[e.Experience] {
			v.fail(ref, "experience: no experience with id %q exists", e.Experience)
		}
		v.yearOrMonth(ref, "start", e.Start)
		v.yearOrMonth(ref, "end", e.End)
		v.engagementEndNotBeforeStart(ref, e.Start, e.End)
	}
}

// tools checks the tools section: the required fields, the enumerated group
// and the forbidden content rule on every public text field.
func (v *validator) tools(entries []ToolEntry) {
	for i, e := range entries {
		ref := entryRef("tools", i, e.ID)
		v.required(ref, "id", e.ID)
		v.required(ref, "name", e.Name)
		v.required(ref, "description", e.Description)
		v.group(ref, e.Group)
		for _, f := range []struct{ field, value string }{
			{"name", e.Name},
			{"description", e.Description},
		} {
			for _, reason := range forbiddenContent(f.value) {
				v.fail(ref, "%s: %s", f.field, reason)
			}
		}
	}
}

// group fails ref when value is empty or none of the five groups.
func (v *validator) group(ref string, value Group) {
	if strings.TrimSpace(string(value)) == "" {
		v.fail(ref, "group: is required")
		return
	}
	if !slices.Contains(groups, value) {
		v.fail(ref, "group: unknown value %q", value)
	}
}

// level fails ref when value is set but is none of the four levels
// docs/concepts/cv.md defines.
func (v *validator) level(ref string, value Level) {
	if strings.TrimSpace(string(value)) == "" {
		v.fail(ref, "level: is required")
		return
	}
	for _, allowed := range levels {
		if value == allowed {
			return
		}
	}
	v.fail(ref, "level: unknown value %q", value)
}

// skillEvidence checks every skill's evidence against the project,
// experience and certificate ids that actually exist in doc.
func (v *validator) skillEvidence(doc Document) {
	existing := map[EvidenceType]map[string]bool{
		EvidenceTypeProject:     toSet(ids(doc.Projects)),
		EvidenceTypeExperience:  toSet(ids(doc.Experience)),
		EvidenceTypeCertificate: toSet(ids(doc.Certificates)),
	}

	for i, s := range doc.Skills {
		for j, e := range s.Evidence {
			ref := fmt.Sprintf("%s.evidence[%d]", entryRef("skills", i, s.ID), j)
			set, known := existing[e.Type]
			if !known {
				v.fail(ref, "type: unknown value %q", e.Type)
				continue
			}
			if strings.TrimSpace(e.Ref) == "" {
				v.fail(ref, "ref: is required")
				continue
			}
			if !set[e.Ref] {
				v.fail(ref, "ref: no %s with id %q exists", e.Type, e.Ref)
			}
		}
	}
}

// entryRef names one entry for an error message: its id when it has one,
// its position in the file otherwise, since a missing id is itself reported
// separately.
func entryRef(section string, index int, id string) string {
	if strings.TrimSpace(id) == "" {
		return fmt.Sprintf("%s[%d]", section, index)
	}
	return fmt.Sprintf("%s[%d] (id=%s)", section, index, id)
}

// duplicates returns every value that appears more than once in values,
// each named once, in the order its second occurrence appears.
func duplicates(values []string) []string {
	seen := make(map[string]bool, len(values))
	reported := make(map[string]bool, len(values))
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			// A missing id is reported by required; it is not this
			// function's duplicate to report, or every entry missing an
			// id would also be flagged as a duplicate of the others.
			continue
		}
		if seen[v] && !reported[v] {
			out = append(out, v)
			reported[v] = true
		}
		seen[v] = true
	}
	return out
}

// toSet turns values into a membership set.
func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}
