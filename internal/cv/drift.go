package cv

import (
	"errors"
	"fmt"
	"sort"
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
	return errors.Join(problems...)
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
