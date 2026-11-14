package cv

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse decodes data into a Document and validates it, aggregating every
// problem from both steps into one error rather than stopping at the
// first: a decode error over an unknown field does not hide the entries
// that did decode, so Validate still runs against whatever the decoder
// managed to fill in, and both sets of problems come back together. source
// names where data came from, for the error messages only.
func Parse(source string, data []byte) (Document, error) {
	var doc Document
	var problems []error

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		problems = append(problems, fmt.Errorf("cv: %s: %w", source, err))
	}

	if err := Validate(doc, source); err != nil {
		problems = append(problems, err)
	}

	return doc, errors.Join(problems...)
}

// Load reads path and parses it into a Document, the same validated model
// Parse returns.
func Load(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("cv: %s: %w", path, err)
	}
	return Parse(path, data)
}

// LoadPair loads dePath and enPath, validates each on its own, and then
// checks the two for drift. Every problem from all three steps is reported
// together: a mistake in one file never hides a mistake in the other, and a
// drift between the two is only skipped when one of the files failed to
// load at all, since there is then nothing meaningful left to compare.
func LoadPair(dePath, enPath string) (de, en Document, err error) {
	var problems []error

	de, deErr := Load(dePath)
	if deErr != nil {
		problems = append(problems, deErr)
	}
	en, enErr := Load(enPath)
	if enErr != nil {
		problems = append(problems, enErr)
	}

	if deErr == nil && enErr == nil {
		if driftErr := Drift(de, en); driftErr != nil {
			problems = append(problems, driftErr)
		}
	}

	return de, en, errors.Join(problems...)
}
