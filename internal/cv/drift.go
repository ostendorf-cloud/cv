package cv

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// Drift compares de (data/cv.de.yaml) and en (data/cv.en.yaml)'s id sets,
// per section, and reports every id present on one side and absent on the
// other. It is a presence check, not a text diff: the two files disagreeing
// in wording is normal, docs/concepts/cv.md's "Two data files, matched by
// identity" settles that disagreeing in which entries exist is the bug.
//
// Every missing id is reported together; Drift never stops at the first
// one.
func Drift(de, en Document) error {
	deIDs := sectionIDs(de)
	enIDs := sectionIDs(en)

	var problems []error
	for _, section := range sectionNames {
		deSet := toSet(deIDs[section])
		enSet := toSet(enIDs[section])

		for _, id := range sortedKeys(deSet) {
			if !enSet[id] {
				problems = append(problems, fmt.Errorf(
					"cv: drift: %s %q is in the German file but not the English one", section, id))
			}
		}
		for _, id := range sortedKeys(enSet) {
			if !deSet[id] {
				problems = append(problems, fmt.Errorf(
					"cv: drift: %s %q is in the English file but not the German one", section, id))
			}
		}
	}
	problems = append(problems, groupingDrift(de, en)...)
	problems = append(problems, toolGroupDrift(de, en)...)
	return errors.Join(problems...)
}

// groupingDrift reports a skill whose category differs, and a category whose
// order differs, between the two files, so the grouping cannot depend on the
// language. Ids present on one side only are reported by Drift itself.
func groupingDrift(de, en Document) []error {
	var problems []error

	enSkills := make(map[string]SkillEntry, len(en.Skills))
	for _, s := range en.Skills {
		enSkills[s.ID] = s
	}
	for _, s := range de.Skills {
		if other, ok := enSkills[s.ID]; ok && s.Category != other.Category {
			problems = append(problems, fmt.Errorf(
				"cv: drift: skills %q has category %q in the German file but %q in the English one",
				s.ID, s.Category, other.Category))
		}
	}

	enCategories := make(map[string]SkillCategoryEntry, len(en.SkillCategories))
	for _, c := range en.SkillCategories {
		enCategories[c.ID] = c
	}
	for _, c := range de.SkillCategories {
		if other, ok := enCategories[c.ID]; ok && orderText(c.Order) != orderText(other.Order) {
			problems = append(problems, fmt.Errorf(
				"cv: drift: skill_categories %q has order %s in the German file but %s in the English one",
				c.ID, orderText(c.Order), orderText(other.Order)))
		}
	}
	return problems
}

// toolGroupDrift reports every tool present in both files whose group differs.
// The group is not text, it is an enumerated value, so the two files must agree.
func toolGroupDrift(de, en Document) []error {
	enGroups := make(map[string]Group, len(en.Tools))
	for _, t := range en.Tools {
		enGroups[t.ID] = t.Group
	}
	var problems []error
	for _, t := range de.Tools {
		if g, ok := enGroups[t.ID]; ok && g != t.Group {
			problems = append(problems, fmt.Errorf(
				"cv: drift: tools %q has group %q in the German file but %q in the English one",
				t.ID, t.Group, g))
		}
	}
	return problems
}

// orderText names an optional order for a message and for comparison.
func orderText(order *int) string {
	if order == nil {
		return "unset"
	}
	return strconv.Itoa(*order)
}

// sortedKeys returns set's keys in a stable order, so Drift's error
// messages do not depend on map iteration order.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
