package cv

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// subjectName is the person the document describes. The data model carries no
// name field, so the PDF header takes it from here.
const subjectName = "Simon Ostendorf"

// PhotoFileName is the name the template reads the portrait from, next to the
// generated source in typst's working directory.
const PhotoFileName = "portrait.jpg"

// fontDir is where CompilePDF writes the bundled fonts, inside typst's
// working directory. typst finds them through --font-path.
const fontDir = "fonts"

//go:embed templates/cv.typ.tmpl
var typstTemplate string

// fonts holds the font files the template names. typst embeds only a serif
// and a monospace font, so the sans font ships here. See
// templates/fonts/LICENSE.
//
//go:embed templates/fonts/*.ttf
var fonts embed.FS

// PDFFileName is the release asset name of the PDF for lang, for example
// cv.en.pdf. The two websites pin it, like JSONFileName.
func PDFFileName(lang string) string {
	return "cv." + lang + ".pdf"
}

// labels holds the fixed texts of one language. They are template data, not
// code: the German texts are what the German PDF prints.
type labels struct {
	Document     string
	AsOf         string
	Present      string
	ValidUntil   string
	Expired      string
	Location     string
	Email        string
	Languages    string
	Profiles     string
	Experience   string
	Engagements  string
	Education    string
	Grade        string
	Certificates string
	Skills       string
	Volunteering string
	Awards       string
	ExamResults  string
	Courses      string
	Publications string
	Patents      string
	Organisation string
	References   string
	PatentNo     string
	LangLevels   map[Level]string
	Months       [12]string
	MonthsLong   [12]string
	Lang         string
}

var pdfLabels = map[string]labels{
	"en": {
		Document: "Curriculum vitae", AsOf: "As of", Present: "present", ValidUntil: "valid until",
		Expired: "expired", Location: "Location", Email: "Email", Languages: "Languages",
		Profiles: "Profiles", Experience: "Experience", Engagements: "Selected engagements",
		Education: "Education", Grade: "Grade", Certificates: "Certificates", Skills: "Skills",
		Volunteering: "Volunteering", Awards: "Honors and awards",
		ExamResults: "Exam results", Courses: "Courses", Publications: "Publications",
		Patents: "Patents", Organisation: "Memberships", References: "References", PatentNo: "No.",
		LangLevels: map[Level]string{
			LevelFamiliar: "Basic", LevelWorking: "Good working knowledge",
			LevelProficient: "Fluent", LevelExpert: "Native",
		},
		Months: [12]string{
			"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
		},
		MonthsLong: [12]string{
			"January", "February", "March", "April", "May", "June", "July", "August", "September",
			"October", "November", "December",
		},
		Lang: "en",
	},
	"de": {
		Document: "Lebenslauf", AsOf: "Stand:", Present: "heute", ValidUntil: "gültig bis",
		Expired: "abgelaufen", Location: "Ort", Email: "E-Mail", Languages: "Sprachen",
		Profiles: "Profile", Experience: "Berufserfahrung", Engagements: "Ausgewählte Einsätze",
		Education: "Ausbildung", Grade: "Note", Certificates: "Zertifikate", Skills: "Kenntnisse",
		Volunteering: "Ehrenamt", Awards: "Auszeichnungen",
		ExamResults: "Prüfungsergebnisse", Courses: "Kurse", Publications: "Veröffentlichungen",
		Patents: "Patente", Organisation: "Mitgliedschaften", References: "Referenzen", PatentNo: "Nr.",
		LangLevels: map[Level]string{
			LevelFamiliar: "Grundkenntnisse", LevelWorking: "Gute Kenntnisse",
			LevelProficient: "Fließend", LevelExpert: "Muttersprache",
		},
		Months: [12]string{
			"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sept.", "Okt.", "Nov.", "Dez.",
		},
		MonthsLong: [12]string{
			"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September",
			"Oktober", "November", "Dezember",
		},
		Lang: "de",
	},
}

// levelRank orders the skills of one category: the highest level first.
var levelRank = map[Level]int{LevelExpert: 4, LevelProficient: 3, LevelWorking: 2, LevelFamiliar: 1}

// pdfLink is one link of the info row: its text and its target.
type pdfLink struct {
	Text string
	URL  string
}

// pdfInfo is one label and value pair of the info row under the header.
// Values holds plain texts, Links links; one of the two is set.
type pdfInfo struct {
	Label  string
	Values []string
	Links  []pdfLink
}

// pdfItem is one printed entry. Dates, Title and the rest are already final
// text; the template only places them. Marker prints as a small outlined
// badge after the title, such as an expired certificate's note.
type pdfItem struct {
	Dates    string
	Title    string
	Subtitle string
	Marker   string
	Body     string
	URL      string
}

// pdfCategory is one skill category with its skill names, highest level
// first.
type pdfCategory struct {
	Name   string
	Skills []string
}

// pdfSection is one printed section. Kind "entries" prints Items as dated
// entries, "skills" prints Categories as groups of pills.
type pdfSection struct {
	Title      string
	Kind       string
	Items      []pdfItem
	Categories []pdfCategory
}

type pdfData struct {
	Lang      string
	Name      string
	Document  string
	Headline  string
	AsOf      string
	Photo     bool
	PhotoFile string
	Info      []pdfInfo
	Sections  []pdfSection
}

// TypstOptions is what a typst render needs besides the data.
type TypstOptions struct {
	// Now is the render date. It decides which certificates print as
	// expired and is the month of the footer's date line.
	Now time.Time
	// Photo places PhotoFileName in the header. CompilePDF writes the file
	// next to the source; without it the header has no photo column.
	Photo bool
}

// RenderTypst renders doc, the data file of language lang, as typst source.
// A certificate whose valid_until month is before the month of opts.Now
// prints as expired. Sections and skill categories without entries are left
// out. It fails when the template names something the model does not hold,
// or when a required text is blank, rather than printing a blank field. Like
// RenderJSON it does not validate doc; the caller does, through Load or
// LoadPair, and everything in doc is printed: nothing is filtered.
func RenderTypst(doc Document, lang string, opts TypstOptions) ([]byte, error) {
	return renderTypst(typstTemplate, doc, lang, opts)
}

func renderTypst(text string, doc Document, lang string, opts TypstOptions) ([]byte, error) {
	l, ok := pdfLabels[lang]
	if !ok {
		return nil, fmt.Errorf("cv: render pdf: unknown language %q", lang)
	}
	data, err := buildPDFData(doc, l, opts)
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

func buildPDFData(doc Document, l labels, opts TypstOptions) (pdfData, error) {
	d := pdfData{
		Lang:      l.Lang,
		Name:      subjectName,
		Document:  l.Document,
		Headline:  headline(doc),
		AsOf:      l.AsOf + " " + l.MonthsLong[opts.Now.Month()-1] + " " + opts.Now.Format("2006"),
		Photo:     opts.Photo,
		PhotoFile: PhotoFileName,
	}

	info, err := infoRow(doc, l)
	if err != nil {
		return pdfData{}, err
	}
	d.Info = info

	add := func(title string, items []pdfItem) {
		if len(items) > 0 {
			d.Sections = append(d.Sections, pdfSection{Title: title, Kind: "entries", Items: items})
		}
	}

	var items []pdfItem
	for _, e := range doc.Experience {
		items = append(items, pdfItem{
			Dates:    l.period(e.Start, e.End, true),
			Title:    e.Title,
			Subtitle: subtitle(e.Title, e.Employer, shortLocation(e.Location), e.EmploymentType),
			Body:     e.Summary,
		})
	}
	add(l.Experience, items)

	employers := make(map[string]string, len(doc.Experience))
	for _, e := range doc.Experience {
		employers[strings.TrimSpace(e.ID)] = e.Employer
	}
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
	add(l.Engagements, items)

	items = nil
	for _, e := range doc.Education {
		items = append(items, pdfItem{
			Dates:    l.period(e.Start, e.End, true),
			Title:    e.Degree,
			Subtitle: joinNonBlank(e.Institution, labelled(l.Grade, e.Grade)),
			Body:     e.Summary,
		})
	}
	add(l.Education, items)

	categories, err := skillCategories(doc)
	if err != nil {
		return pdfData{}, err
	}
	if len(categories) > 0 {
		d.Sections = append(d.Sections, pdfSection{Title: l.Skills, Kind: "skills", Categories: categories})
	}

	items = nil
	currentMonth := opts.Now.Format("2006-01")
	for _, e := range doc.Certificates {
		item := pdfItem{Dates: l.month(e.Issued), Title: e.Name, Subtitle: e.Issuer}
		if v := strings.TrimSpace(e.ValidUntil); v != "" {
			if v < currentMonth {
				item.Marker = l.Expired + " " + l.month(v)
			} else {
				item.Subtitle = joinNonBlank(e.Issuer, l.ValidUntil+" "+l.month(v))
			}
		}
		items = append(items, item)
	}
	add(l.Certificates, items)

	items = nil
	for _, e := range doc.ExamResults {
		items = append(items, pdfItem{
			Dates: l.month(e.Date), Title: e.Name, Subtitle: e.Score, Body: e.Summary,
		})
	}
	add(l.ExamResults, items)

	// One line per award: title, issuer and date. The summary stays in the
	// JSON and on the websites.
	items = nil
	for _, e := range doc.Awards {
		items = append(items, pdfItem{
			Dates: l.month(e.Date), Title: e.Title, Subtitle: e.Issuer,
		})
	}
	add(l.Awards, items)

	// The repository projects are not printed: the websites show them with
	// their GitHub data, and the PDF has a page budget. The engagements above
	// are the projects of the positions.

	// Volunteering prints title, organisation and the short description. The
	// cause, such as "Politik", is left out.
	items = nil
	for _, e := range doc.Volunteering {
		items = append(items, pdfItem{
			Dates:    l.period(e.Start, e.End, true),
			Title:    e.Role,
			Subtitle: strings.TrimSpace(e.Organisation),
			Body:     e.Summary,
		})
	}
	add(l.Volunteering, items)

	items = nil
	for _, e := range doc.Courses {
		items = append(items, pdfItem{Dates: l.month(e.Date), Title: e.Name, Subtitle: e.Provider})
	}
	add(l.Courses, items)

	items = nil
	for _, e := range doc.Publications {
		items = append(items, pdfItem{
			Dates: l.month(e.Date), Title: e.Title, Subtitle: e.Publisher, Body: e.Summary, URL: e.URL,
		})
	}
	add(l.Publications, items)

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
	add(l.Patents, items)

	items = nil
	for _, e := range doc.Organisations {
		items = append(items, pdfItem{
			Dates: l.period(e.Start, e.End, true), Title: e.Name, Subtitle: e.Role,
		})
	}
	add(l.Organisation, items)

	items = nil
	for _, e := range doc.References {
		items = append(items, pdfItem{Title: e.Name, Subtitle: e.Role})
	}
	add(l.References, items)

	return d, nil
}

// headline is the line under the name. The model has no headline field, so
// it is the title of the first ongoing position, in file order, and empty
// when no position is ongoing.
func headline(doc Document) string {
	for _, e := range doc.Experience {
		if strings.TrimSpace(e.End) == "" {
			return strings.TrimSpace(e.Title)
		}
	}
	return ""
}

// infoRow is the label and value pairs under the header: location, email,
// languages and profiles, each left out when the data holds nothing for it.
// The model has no phone number, so the row has none.
func infoRow(doc Document, l labels) ([]pdfInfo, error) {
	var row []pdfInfo
	if a := doc.Contact.Address; a != nil {
		if location := joinWith(", ", a.Street, a.City); location != "" {
			row = append(row, pdfInfo{Label: l.Location, Values: []string{location}})
		}
	}
	if email := strings.TrimSpace(doc.Contact.Email); email != "" {
		row = append(row, pdfInfo{Label: l.Email, Links: []pdfLink{{Text: email, URL: "mailto:" + email}}})
	}
	var languages []string
	for _, e := range doc.Languages {
		level, ok := l.LangLevels[Level(strings.TrimSpace(string(e.Level)))]
		if !ok {
			return nil, fmt.Errorf("cv: render pdf: language %q has the unknown level %q", e.ID, e.Level)
		}
		languages = append(languages, strings.TrimSpace(e.Name)+" ("+level+")")
	}
	if len(languages) > 0 {
		row = append(row, pdfInfo{Label: l.Languages, Values: languages})
	}
	var links []pdfLink
	for _, p := range doc.Contact.Profiles {
		links = append(links, pdfLink{Text: p.Platform, URL: p.URL})
	}
	if len(links) > 0 {
		row = append(row, pdfInfo{Label: l.Profiles, Links: links})
	}
	return row, nil
}

// skillCategories groups the skills under their categories in display order,
// the order the JSON uses. Within a category the highest level comes first,
// equal levels in file order. A category without skills is left out.
func skillCategories(doc Document) ([]pdfCategory, error) {
	type ranked struct {
		name string
		rank int
	}
	byCategory := make(map[string][]ranked, len(doc.SkillCategories))
	known := make(map[string]bool, len(doc.SkillCategories))
	for _, c := range doc.SkillCategories {
		known[strings.TrimSpace(c.ID)] = true
	}
	for _, s := range doc.Skills {
		rank, ok := levelRank[Level(strings.TrimSpace(string(s.Level)))]
		if !ok {
			return nil, fmt.Errorf("cv: render pdf: skill %q has the unknown level %q", s.ID, s.Level)
		}
		category := strings.TrimSpace(s.Category)
		if !known[category] {
			return nil, fmt.Errorf("cv: render pdf: skill %q names the unknown category %q", s.ID, s.Category)
		}
		byCategory[category] = append(byCategory[category], ranked{name: s.Name, rank: rank})
	}

	var out []pdfCategory
	for _, c := range sortedCategories(doc.SkillCategories) {
		skills := byCategory[strings.TrimSpace(c.ID)]
		if len(skills) == 0 {
			continue
		}
		sort.SliceStable(skills, func(i, j int) bool { return skills[i].rank > skills[j].rank })
		names := make([]string, len(skills))
		for i, s := range skills {
			names[i] = s.name
		}
		out = append(out, pdfCategory{Name: c.Name, Skills: names})
	}
	return out, nil
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

// period joins start and end with an en dash. An end equal to the start
// prints the start alone. An open end prints as the word for present when
// ongoing is set, and prints the start alone otherwise.
func (l labels) period(start, end string, ongoing bool) string {
	start, end = strings.TrimSpace(start), strings.TrimSpace(end)
	switch {
	case start == "":
		return l.month(end)
	case end == start:
		return l.month(start)
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

// shortLocation is the part of a location before its first comma, so
// "Städteregion Aachen, Nordrhein-Westfalen, Deutschland, Hybrid" prints as
// "Städteregion Aachen". The full value stays in the JSON.
func shortLocation(location string) string {
	head, _, _ := strings.Cut(location, ",")
	return strings.TrimSpace(head)
}

// subtitle joins the non blank parts with a middle dot and leaves out a part
// that repeats the title or an earlier part, ignoring case, so an employment
// type "Selbstständig" under the employer "Selbstständig" prints once.
func subtitle(title string, parts ...string) string {
	seen := []string{strings.TrimSpace(title)}
	var kept []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		repeat := false
		for _, s := range seen {
			if strings.EqualFold(p, s) {
				repeat = true
				break
			}
		}
		if repeat {
			continue
		}
		seen = append(seen, p)
		kept = append(kept, p)
	}
	return strings.Join(kept, " · ")
}

// joinNonBlank joins the non blank parts with a middle dot, the separator
// the template uses between a title and its organisation.
func joinNonBlank(parts ...string) string {
	return joinWith(" · ", parts...)
}

func joinWith(sep string, parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

// LoadPhoto reads the portrait at path. It fails with an error that names
// the path when the file is missing or is not a JPEG, so a render never
// prints an empty photo frame.
func LoadPhoto(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cv: render pdf: read the portrait: %w (pass --photo \"\" to render without one)", err)
	}
	if !bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) {
		return nil, fmt.Errorf("cv: render pdf: the portrait %s is not a JPEG", path)
	}
	return data, nil
}

// CompilePDF compiles typst source to PDF bytes with the typst binary at
// typstBin, which is "typst" resolved through PATH when empty. The binary
// comes from mise: see mise.toml. It writes the source, the bundled fonts
// and, when photo is not nil, the portrait as PhotoFileName into a temporary
// directory, passes the fonts with --font-path and ignores the system fonts.
func CompilePDF(ctx context.Context, typstBin string, source, photo []byte) ([]byte, error) {
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

	fontPath := filepath.Join(dir, fontDir)
	if err := writeFonts(fontPath); err != nil {
		return nil, fmt.Errorf("cv: render pdf: write the fonts: %w", err)
	}
	if photo != nil {
		if err := os.WriteFile(filepath.Join(dir, PhotoFileName), photo, 0o600); err != nil {
			return nil, fmt.Errorf("cv: render pdf: %w", err)
		}
	}

	in := filepath.Join(dir, "cv.typ")
	out := filepath.Join(dir, "cv.pdf")
	if err := os.WriteFile(in, source, 0o600); err != nil {
		return nil, fmt.Errorf("cv: render pdf: %w", err)
	}

	var stderr bytes.Buffer
	// --ignore-system-fonts: only the bundled fonts and the ones typst embeds
	// are candidates, so no text can fall back to a font the machine happens
	// to have, such as a runner's Liberation Sans.
	cmd := exec.CommandContext(ctx, bin, "compile", "--font-path", fontPath, in, out, "--ignore-system-fonts")
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

// writeFonts copies the embedded font files into dir.
func writeFonts(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	entries, err := fs.ReadDir(fonts, "templates/fonts")
	if err != nil {
		return err
	}
	for _, e := range entries {
		data, err := fonts.ReadFile("templates/fonts/" + e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}
