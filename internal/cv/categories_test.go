package cv

import (
	"fmt"
	"strings"
	"testing"
)

// TestSortedCategories covers the documented order rule: categories with an
// order first, ascending, equal orders in file order, then the categories
// without one in file order. The fixture has more than 12 entries, so an
// unstable sort would reorder equal values.
func TestSortedCategories(t *testing.T) {
	t.Parallel()

	var in []SkillCategoryEntry
	in = append(in, SkillCategoryEntry{ID: "unset-first"})
	for i := range 14 {
		order := 7 - i/2 // 7 7 6 6 ... 1 1: descending, in pairs of equal values
		in = append(in, SkillCategoryEntry{ID: fmt.Sprintf("o%02d-order%d", i, order), Order: &order})
		if i == 6 {
			in = append(in, SkillCategoryEntry{ID: "unset-middle"})
		}
	}

	var want []string
	for order := 1; order <= 7; order++ {
		for _, e := range in {
			if e.Order != nil && *e.Order == order {
				want = append(want, e.ID)
			}
		}
	}
	want = append(want, "unset-first", "unset-middle")

	var got []string
	for _, e := range sortedCategories(in) {
		got = append(got, e.ID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("sortedCategories() =\n%v\nwant\n%v", got, want)
	}
	if in[0].ID != "unset-first" {
		t.Errorf("sortedCategories changed its input: first entry = %q", in[0].ID)
	}
}

// TestParseCategoryOrder proves a non integer order is refused with one clear
// error and no follow up error about the value it left behind.
func TestParseCategoryOrder(t *testing.T) {
	t.Parallel()

	const head = "skill_categories:\n  - id: cloud\n    name: Cloud\n"
	const tail = "contact:\n  email: contact@example.com\n"

	tests := map[string]struct {
		order   string
		want    string // empty means valid
		wantNot string
	}{
		"a whole number is valid":         {order: "    order: 2\n"},
		"no order is valid":               {order: ""},
		"a fraction is refused":           {order: "    order: 1.5\n", want: "order \"1.5\" is not a whole number", wantNot: "positive"},
		"a string is refused":             {order: "    order: \"1\"\n", want: "order \"1\" is not a whole number", wantNot: "positive"},
		"zero is refused as not positive": {order: "    order: 0\n", want: "order: 0 is not a positive number"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse("test", []byte(head+tc.order+tail))
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Parse() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse() error = %v, want it to contain %q", err, tc.want)
			}
			if tc.wantNot != "" && strings.Contains(err.Error(), tc.wantNot) {
				t.Errorf("Parse() error = %v, must not contain %q", err, tc.wantNot)
			}
		})
	}
}
