// Package auditschema renders the audit data portion of the envelope schema.
package auditschema

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

type node map[string]any

const (
	typeKey    = "type"
	refKey     = "$ref"
	patternKey = "pattern"
)

type spec struct {
	Defaults map[string]field `json:"defaults"`
	KeyRules map[string]field `json:"keyRules"`
	Types    map[string]event `json:"types"`
}

type event struct {
	Keys map[string]field `json:"keys"`
}

type field struct {
	Type           string           `json:"type"`
	Ref            string           `json:"ref"`
	Required       bool             `json:"required"`
	RequiredWhen   *condition       `json:"requiredWhen"`
	RequiredUnless *condition       `json:"requiredUnless"`
	ForbiddenWhen  *condition       `json:"forbiddenWhen"`
	ByKey          string           `json:"byKey"`
	Variants       map[string]field `json:"variants"`
	Pattern        string           `json:"pattern"`
	Format         string           `json:"format"`
	Enum           []string         `json:"enum"`
	MinBytes       int              `json:"minBytes"`
	MaxBytes       int              `json:"maxBytes"`
	Minimum        *int             `json:"minimum"`
	Maximum        *int             `json:"maximum"`
	AllowNegative  bool             `json:"allowNegative"`
	MaxItems       int              `json:"maxItems"`
	Items          *field           `json:"items"`
	Const          *bool            `json:"const"`
}

// Render converts the shared allow-list into a deterministic JSON Schema.
func Render(raw []byte) ([]byte, error) {
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("read audit allow-list: %w", err)
	}
	if len(s.Types) == 0 {
		return nil, fmt.Errorf("audit allow-list has no event types")
	}
	doc, err := s.document()
	if err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode audit schema: %w", err)
	}
	return append(out, '\n'), nil
}

func (s spec) document() (node, error) {
	defs := node{}
	var cases []node
	for _, typ := range slices.Sorted(maps.Keys(s.Types)) {
		data, err := s.event(s.Types[typ])
		if err != nil {
			return nil, fmt.Errorf("audit type %s: %w", typ, err)
		}
		defs[typ] = data
		cases = append(cases, implication(
			condition{Key: typeKey, Equals: typ}.schema(),
			property("data", node{refKey: "#/$defs/" + typ}),
		))
	}
	return node{
		"$schema":  "https://json-schema.org/draft/2020-12/schema",
		"$id":      "https://github.com/devsy-org/devsy-signals/schema/audit-data.schema.json",
		"$comment": "Generated from audit-allowlist.json. Do not edit.",
		"$defs":    defs,
		"allOf":    cases,
	}, nil
}

func (s spec) event(e event) (node, error) {
	props := node{}
	var required []string
	var cases []node
	for _, key := range slices.Sorted(maps.Keys(e.Keys)) {
		f, err := s.resolve(e.Keys[key])
		if err != nil {
			return nil, fmt.Errorf("audit key %s: %w", key, err)
		}
		props[key], err = s.value(f)
		if err != nil {
			return nil, fmt.Errorf("audit key %s: %w", key, err)
		}
		if f.Required {
			required = append(required, key)
		}
		variants, err := s.variants(key, f)
		if err != nil {
			return nil, fmt.Errorf("audit key %s: %w", key, err)
		}
		cases = append(cases, f.conditions(key)...)
		cases = append(cases, variants...)
	}
	out := node{typeKey: "object", "additionalProperties": false, "properties": props}
	if len(required) != 0 {
		out["required"] = required
	}
	if len(cases) != 0 {
		out["allOf"] = cases
	}
	return out, nil
}

func (s spec) resolve(f field) (field, error) {
	if f.Ref == "" {
		return f, nil
	}
	rule, ok := s.KeyRules[f.Ref]
	if !ok {
		return field{}, fmt.Errorf("unknown audit rule %q", f.Ref)
	}
	return rule, nil
}
