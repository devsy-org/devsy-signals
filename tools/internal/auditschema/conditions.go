package auditschema

import (
	"maps"
	"slices"
)

type condition struct {
	Key     string `json:"key"`
	Equals  any    `json:"equals"`
	Present string `json:"present"`
	Absent  string `json:"absent"`
}

func (c condition) schema() node {
	switch {
	case c.Present != "":
		return requires(c.Present)
	case c.Absent != "":
		return negate(requires(c.Absent))
	default:
		out := requires(c.Key)
		out["properties"] = node{c.Key: node{"const": c.Equals}}
		return out
	}
}

func (f field) conditions(key string) []node {
	var cases []node
	if f.RequiredWhen != nil {
		cases = append(cases, implication(f.RequiredWhen.schema(), requires(key)))
	}
	if f.RequiredUnless != nil {
		cases = append(cases, implication(negate(f.RequiredUnless.schema()), requires(key)))
	}
	if f.ForbiddenWhen != nil {
		cases = append(cases, implication(f.ForbiddenWhen.schema(), negate(requires(key))))
	}
	return cases
}

func (s spec) variants(key string, f field) ([]node, error) {
	var cases []node
	for _, label := range slices.Sorted(maps.Keys(f.Variants)) {
		variant := f.Variants[label]
		variant.Type = f.Type
		value, err := s.value(variant)
		if err != nil {
			return nil, err
		}
		then := property(key, value)
		if variant.Required {
			then["required"] = []string{key}
		}
		cases = append(cases, implication(condition{Key: f.ByKey, Equals: label}.schema(), then))
	}
	return cases, nil
}

func requires(key string) node              { return node{"required": []string{key}} }
func negate(schema node) node               { return node{"not": schema} }
func property(key string, schema node) node { return node{"properties": node{key: schema}} }
func implication(test, then node) node      { return node{"if": test, "then": then} }
