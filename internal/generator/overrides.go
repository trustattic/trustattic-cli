package generator

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Override replaces the mechanically-derived Group and/or Verb for one
// operationId. A zero-value field (nil Group, empty Verb) leaves that part
// of the mechanical result untouched.
type Override struct {
	Group []string `yaml:"group,omitempty"`
	Verb  string   `yaml:"verb,omitempty"`
}

// LoadOverrides reads operationId -> Override entries from path. A missing
// file is not an error; it returns an empty map.
func LoadOverrides(path string) (map[string]Override, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Override{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read overrides: %w", err)
	}
	var overrides map[string]Override
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("parse overrides: %w", err)
	}
	if overrides == nil {
		overrides = map[string]Override{}
	}
	return overrides, nil
}

// ApplyOverrides returns specs with each entry's Group/Verb replaced by its
// matching override, if any (matched by operationId).
func ApplyOverrides(specs []CommandSpec, overrides map[string]Override) []CommandSpec {
	out := make([]CommandSpec, len(specs))
	for i, s := range specs {
		o, ok := overrides[s.Operation.OperationID]
		if !ok {
			out[i] = s
			continue
		}
		if o.Group != nil {
			s.Group = o.Group
		}
		if o.Verb != "" {
			s.Verb = o.Verb
		}
		out[i] = s
	}
	return out
}
