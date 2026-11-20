package cv

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// subjectName is the person the document describes. The data model carries no
// name field, so the PDF header takes it from here.
const subjectName = "Simon Ostendorf"

//go:embed templates/cv.typ.tmpl
var typstTemplate string

// PDFFileName is the release asset name of the PDF for lang, for example
// cv.en.pdf. The two websites pin it, like JSONFileName.
func PDFFileName(lang string) string {
	return "cv." + lang + ".pdf"
}

// labels holds the fixed texts of one language. They are template data, not
// code: the German texts are what the German PDF prints.
type labels struct {
	Document     string
	Present      string
	ValidUntil   string
	Expired      string
	Experience   string
	Engagements  string
	Education    string
	Grade        string
	Certificates string
	Skills       string
	Languages    string
	Projects     string
	Volunteering string
	Awards       string
	ExamResults  string
	Courses      string
	Publications string
	Patents      string
	Organisation string
	References   string
	PatentNo     string
	SkillLevels  map[Level]string
	LangLevels   map[Level]string
	Months       [12]string
	HTMLLang     string
}

var pdfLabels = map[string]labels{
	"en": {
		Document: "Curriculum vitae", Present: "present", ValidUntil: "valid until",
		Expired: "expired", Experience: "Experience", Engagements: "Selected engagements",
		Education: "Education", Grade: "Grade", Certificates: "Certificates", Skills: "Skills",
		Languages: "Languages", Projects: "Projects", Volunteering: "Volunteering",
		Awards: "Honors and awards", ExamResults: "Exam results", Courses: "Courses",
		Publications: "Publications", Patents: "Patents", Organisation: "Memberships",
		References: "References", PatentNo: "No.",
		SkillLevels: map[Level]string{
			LevelFamiliar: "Familiar", LevelWorking: "Working knowledge",
			LevelProficient: "Proficient", LevelExpert: "Expert",
		},
		LangLevels: map[Level]string{
			LevelFamiliar: "Basic", LevelWorking: "Conversational",
			LevelProficient: "Fluent", LevelExpert: "Expert",
		},
		Months: [12]string{
			"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
		},
		HTMLLang: "en",
	},
	"de": {
		Document: "Lebenslauf", Present: "heute", ValidUntil: "gültig bis",
		Expired: "abgelaufen", Experience: "Berufserfahrung", Engagements: "Ausgewählte Einsätze",
		Education: "Ausbildung", Grade: "Note", Certificates: "Zertifikate", Skills: "Kenntnisse",
		Languages: "Sprachen", Projects: "Projekte", Volunteering: "Ehrenamt",
		Awards: "Auszeichnungen", ExamResults: "Prüfungsergebnisse", Courses: "Kurse",
		Publications: "Veröffentlichungen", Patents: "Patente", Organisation: "Mitgliedschaften",
		References: "Referenzen", PatentNo: "Nr.",
		SkillLevels: map[Level]string{
			LevelFamiliar: "Grundkenntnisse", LevelWorking: "Gute Kenntnisse",
			LevelProficient: "Sehr gute Kenntnisse", LevelExpert: "Expertenkenntnisse",
		},
		LangLevels: map[Level]string{
			LevelFamiliar: "Grundkenntnisse", LevelWorking: "Gute Kenntnisse",
			LevelProficient: "Fließend", LevelExpert: "Exzellente Kenntnisse",
		},
		Months: [12]string{
			"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sept.", "Okt.", "Nov.", "Dez.",
		},
		HTMLLang: "de",
	},
}

// pdfItem is one printed entry. Dates, Title and the rest are already final
// text; the template only places them.
type pdfItem struct {
	Dates    string
	Title    string
	Subtitle string
	Body     string
	URL      string
	Detail   string
}

// pdfSection is one printed section. Kind "entries" prints dated entries,
// "pairs" prints a two column list of Title and Detail.
type pdfSection struct {
	Title string
	Kind  string
	Items []pdfItem
}

type pdfProfile struct {
	Platform string
	URL      string
}

type pdfData struct {
	Lang     string
	Name     string
	Document string
	Email    string
	Address  string
	Profiles []pdfProfile
	Sections []pdfSection
}

// RenderTypst renders doc, the data file of language lang, as typst source.
// now decides which certificates print as expired: a certificate whose
// valid_until month is before the month of now. Sections without entries are
// left out. It fails when the template names something the model does not
// hold, or when a required text is blank, rather than printing a blank field.
// Like RenderJSON it does not validate doc; the caller does, through Load or
// LoadPair, and everything in doc is printed: nothing is filtered.
func RenderTypst(doc Document, lang string, now time.Time) ([]byte, error) {
	return renderTypst(typstTemplate, doc, lang, now)
}

func renderTypst(text string, doc Document, lang string, now time.Time) ([]byte, error) {
	l, ok := pdfLabels[lang]
	if !ok {
		return nil, fmt.Errorf("cv: render pdf: unknown language %q", lang)
	}
	data, err := buildPDFData(doc, l, now)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("cv.typ").Option("missingkey=error").Funcs(template.FuncMap{
		"s": typstRequired,
		"q": typstString,
	}).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("cv: render pdf: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("cv: render pdf (%s): %w", lang, err)
	}
	return buf.Bytes(), nil
}

// typstString writes s as a typst string literal. Whitespace runs, newlines
// included, collapse to one space, so a folded YAML value cannot break the
// source. Inside a string literal nothing but the backslash and the double
// quote is special, so no markup in the data can reach typst.
func typstString(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u{%x}`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// typstRequired is typstString for a text the layout cannot do without: a
// blank value is an error, which fails the whole render.
func typstRequired(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", errors.New("a required text is blank")
	}
	return typstString(s), nil
}

func buildPDFData(doc Document, l labels, now time.Time) (pdfData, error) {
	employers := make(map[string]string, len(doc.Experience))
	for _, e := range doc.Experience {
		employers[strings.TrimSpace(e.ID)] = e.Employer
	}

	d := pdfData{Lang: l.HTMLLang, Name: subjectName, Document: l.Document, Email: doc.Contact.Email}
	if a := doc.Contact.Address; a != nil {
		d.Address = joinNonBlank(a.Street, a.City)
	}
	for _, p := range doc.Contact.Profiles {
		d.Profiles = append(d.Profiles, pdfProfile{Platform: p.Platform, URL: p.URL})
	}

	add := func(title, kind string, items []pdfItem) {
		if len(items) > 0 {
			d.Sections = append(d.Sections, pdfSection{Title: title, Kind: kind, Items: items})
		}
	}

	var items []pdfItem
	for _, e := range doc.Experience {
		items = append(items, pdfItem{
			Dates:    l.period(e.Start, e.End, true),
			Title:    e.Title,
			Subtitle: joinNonBlank(e.Employer, e.Location, e.EmploymentType),
			Body:     e.Summary,
		})
	}
	add(l.Experience, "entries", items)

	items = nil
	for _, e := range doc.Engagements {
		employer, ok := employers[strings.TrimSpace(e.Experience)]
		if !ok {
			return pdfData{}, fmt.Errorf("cv: render pdf: engagement %q names the unknown experience %q", e.ID, e.Experience)
		}
		items = append(items, pdfItem{
			Dates:    l.period(e.Start, e.End, false),
			Title:    e.Title,
			Subtitle: joinNonBlank(e.Industry, employer),
			Body:     e.Summary,
		})
	}
	add(l.Engagements, "entries", items)

	items = nil
	for _, e := range doc.Education {
		items = append(items, pdfItem{
			Dates:    l.period(e.Start, e.End, true),
			Title:    e.Degree,
			Subtitle: joinNonBlank(e.Institution, labelled(l.Grade, e.Grade)),
			Body:     e.Summary,
		})
	}
	add(l.Education, "entries", items)

	items = nil
	currentMonth := now.Format("2006-01")
	for _, e := range doc.Certificates {
		status := ""
		if v := strings.TrimSpace(e.ValidUntil); v != "" {
			if v < currentMonth {
				status = l.Expired + " " + l.month(v)
			} else {
				status = l.ValidUntil + " " + l.month(v)
			}
		}
		items = append(items, pdfItem{
			Dates:    l.month(e.Issued),
			Title:    e.Name,
			Subtitle: joinNonBlank(e.Issuer, status),
		})
	}
	add(l.Certificates, "entries", items)

	items = nil
	for _, e := range doc.Skills {
		level, ok := l.SkillLevels[e.Level]
		if !ok {
			return pdfData{}, fmt.Errorf("cv: render pdf: skill %q has the unknown level %q", e.ID, e.Level)
		}
		items = append(items, pdfItem{Title: e.Name, Detail: level})
	}
	add(l.Skills, "pairs", items)

	items = nil
	for _, e := range doc.Languages {
		level, ok := l.LangLevels[e.Level]
		if !ok {
			return pdfData{}, fmt.Errorf("cv: render pdf: language %q has the unknown level %q", e.ID, e.Level)
		}
		items = append(items, pdfItem{Title: e.Name, Detail: level})
	}
	add(l.Languages, "pairs", items)

	items = nil
	for _, e := range doc.Projects {
		items = append(items, pdfItem{Title: e.GithubRepo, Body: e.Description})
	}
	add(l.Projects, "entries", items)

	items = nil
	for _, e := range doc.Volunteering {
		items = append(items, pdfItem{
			Dates:    l.period(e.Start, e.End, true),
			Title:    e.Role,
			Subtitle: joinNonBlank(e.Organisation, e.Cause),
			Body:     e.Summary,
		})
	}
	add(l.Volunteering, "entries", items)

	items = nil
	for _, e := range doc.Awards {
		items = append(items, pdfItem{
			Dates: l.month(e.Date), Title: e.Title, Subtitle: e.Issuer, Body: e.Summary,
		})
	}
	add(l.Awards, "entries", items)

	items = nil
	for _, e := range doc.ExamResults {
		items = append(items, pdfItem{
			Dates: l.month(e.Date), Title: e.Name, Subtitle: e.Score, Body: e.Summary,
		})
	}
	add(l.ExamResults, "entries", items)

	items = nil
	for _, e := range doc.Courses {
		items = append(items, pdfItem{Dates: l.month(e.Date), Title: e.Name, Subtitle: e.Provider})
	}
	add(l.Courses, "entries", items)

	items = nil
	for _, e := range doc.Publications {
		items = append(items, pdfItem{
			Dates: l.month(e.Date), Title: e.Title, Subtitle: e.Publisher, Body: e.Summary, URL: e.URL,
		})
	}
	add(l.Publications, "entries", items)

	items = nil
	for _, e := range doc.Patents {
		items = append(items, pdfItem{
			Dates:    l.month(e.Date),
			Title:    e.Title,
			Subtitle: labelled(l.PatentNo, e.Number),
			Body:     e.Summary,
			URL:      e.URL,
		})
	}
	add(l.Patents, "entries", items)

	items = nil
	for _, e := range doc.Organisations {
		items = append(items, pdfItem{
			Dates: l.period(e.Start, e.End, true), Title: e.Name, Subtitle: e.Role,
		})
	}
	add(l.Organisation, "entries", items)

	items = nil
	for _, e := range doc.References {
		items = append(items, pdfItem{Title: e.Name, Detail: e.Role})
	}
	add(l.References, "pairs", items)

	return d, nil
}

// month turns "2020-03" into "Mar 2020" or "März 2020". A "YYYY" value and
// an empty one pass through. Anything else is returned as it is: validation
// owns the date format, not the layout.
func (l labels) month(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[4] != '-' {
		return s
	}
	var m int
	if _, err := fmt.Sscanf(s[5:], "%02d", &m); err != nil || m < 1 || m > 12 {
		return s
	}
	return l.Months[m-1] + " " + s[:4]
}

// period joins start and end with an en dash. An open end prints as the word
// for present when ongoing is set, and prints the start alone otherwise.
func (l labels) period(start, end string, ongoing bool) string {
	start, end = strings.TrimSpace(start), strings.TrimSpace(end)
	switch {
	case start == "":
		return l.month(end)
	case end != "":
		return l.month(start) + " – " + l.month(end)
	case ongoing:
		return l.month(start) + " – " + l.Present
	default:
		return l.month(start)
	}
}

func labelled(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return label + " " + strings.TrimSpace(value)
}

// joinNonBlank joins the non blank parts with a comma.
func joinNonBlank(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

// CompilePDF compiles typst source to PDF bytes with the typst binary at
// typstBin, which is "typst" resolved through PATH when empty. The binary
// comes from mise: see mise.toml.
func CompilePDF(ctx context.Context, typstBin string, source []byte) ([]byte, error) {
	if typstBin == "" {
		typstBin = "typst"
	}
	bin, err := exec.LookPath(typstBin)
	if err != nil {
		return nil, fmt.Errorf("cv: render pdf: typst not found (run through mise, see mise.toml): %w", err)
	}

	dir, err := os.MkdirTemp("", "cv-pdf-")
	if err != nil {
		return nil, fmt.Errorf("cv: render pdf: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	in := filepath.Join(dir, "cv.typ")
	out := filepath.Join(dir, "cv.pdf")
	if err := os.WriteFile(in, source, 0o600); err != nil {
		return nil, fmt.Errorf("cv: render pdf: %w", err)
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "compile", in, out)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cv: render pdf: typst compile: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	pdf, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("cv: render pdf: %w", err)
	}
	return pdf, nil
}
