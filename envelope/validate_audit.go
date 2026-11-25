package envelope

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"sync"

	"github.com/devsy-org/devsy-signals/schema"
)

//nolint:gochecknoglobals // All Validate callers share this immutable, lazily compiled contract.
var cachedAuditSpec = sync.OnceValues(loadAuditSpec)

type auditSpec struct {
	Types    map[string]auditEvent `json:"types"`
	KeyRules map[string]auditRule  `json:"keyRules"`
	Defaults map[string]auditRule  `json:"defaults"`
}

type auditEvent struct {
	Keys       map[string]auditRule `json:"keys"`
	sortedKeys []string
}

type auditRule struct {
	Type            string               `json:"type"`
	Ref             string               `json:"ref"`
	Required        bool                 `json:"required"`
	RequiredWhen    *auditCondition      `json:"requiredWhen"`
	RequiredUnless  *auditCondition      `json:"requiredUnless"`
	ForbiddenWhen   *auditCondition      `json:"forbiddenWhen"`
	ByKey           string               `json:"byKey"`
	Variants        map[string]auditRule `json:"variants"`
	Pattern         string               `json:"pattern"`
	Format          string               `json:"format"`
	Enum            []string             `json:"enum"`
	MinBytes        int                  `json:"minBytes"`
	MaxBytes        int                  `json:"maxBytes"`
	Minimum         *int                 `json:"minimum"`
	Maximum         *int                 `json:"maximum"`
	AllowNegative   bool                 `json:"allowNegative"`
	MaxItems        int                  `json:"maxItems"`
	Items           *auditRule           `json:"items"`
	Const           *bool                `json:"const"`
	compiledPattern *regexp.Regexp
}

type auditCondition struct {
	Key     string `json:"key"`
	Equals  any    `json:"equals"`
	Present string `json:"present"`
	Absent  string `json:"absent"`
}

func (e Envelope) checkAuditData() error {
	if e.Data == nil {
		return nil
	}
	spec, err := cachedAuditSpec()
	if err != nil {
		return err
	}
	event, ok := spec.Types[e.Type]
	if !ok {
		return fmt.Errorf("no audit data rules for type %q", e.Type)
	}
	return spec.checkData(event, e.Data)
}

func loadAuditSpec() (auditSpec, error) {
	raw, err := schema.AuditAllowlist()
	if err != nil {
		return auditSpec{}, err
	}
	var spec auditSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return auditSpec{}, fmt.Errorf("decode audit allow-list: %w", err)
	}
	for kind, event := range spec.Types {
		compiled, err := spec.compileEvent(event)
		if err != nil {
			return auditSpec{}, fmt.Errorf("compile audit type %q: %w", kind, err)
		}
		spec.Types[kind] = compiled
	}
	return spec, nil
}

func (s auditSpec) compileEvent(event auditEvent) (auditEvent, error) {
	event.sortedKeys = slices.Sorted(maps.Keys(event.Keys))
	for _, key := range event.sortedKeys {
		rule, ok := s.resolve(event.Keys[key])
		if !ok {
			return auditEvent{}, fmt.Errorf("key %q has no referenced allow-list rule", key)
		}
		compiled, err := compileAuditRule(rule)
		if err != nil {
			return auditEvent{}, fmt.Errorf("key %q: %w", key, err)
		}
		event.Keys[key] = compiled
	}
	return event, nil
}

func compileAuditRule(rule auditRule) (auditRule, error) {
	if rule.Pattern != "" {
		pattern, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return auditRule{}, fmt.Errorf("compile audit pattern: %w", err)
		}
		rule.compiledPattern = pattern
	}
	if rule.Items != nil {
		item, err := compileAuditRule(*rule.Items)
		if err != nil {
			return auditRule{}, err
		}
		rule.Items = &item
	}
	for kind, variant := range rule.Variants {
		compiled, err := compileAuditRule(variant)
		if err != nil {
			return auditRule{}, err
		}
		rule.Variants[kind] = compiled
	}
	return rule, nil
}

func (s auditSpec) resolve(rule auditRule) (auditRule, bool) {
	if rule.Ref == "" {
		return rule, true
	}
	out, ok := s.KeyRules[rule.Ref]
	return out, ok
}

func (s auditSpec) checkData(event auditEvent, data map[string]any) error {
	var p problems
	for _, key := range unknownAuditKeys(event.Keys, data) {
		p.add(false, "data[%q] is not allowed", key)
	}
	for _, key := range event.sortedKeys {
		if !s.validField(event.Keys[key], key, data) {
			p.add(false, "data[%q] violates the audit allow-list", key)
		}
	}
	return errors.Join(p...)
}

func unknownAuditKeys(keys map[string]auditRule, data map[string]any) []string {
	var unknown []string
	for key := range data {
		if _, allowed := keys[key]; !allowed {
			unknown = append(unknown, key)
		}
	}
	slices.Sort(unknown)
	return unknown
}

func (s auditSpec) validField(rule auditRule, key string, data map[string]any) bool {
	rule = rule.forData(data)
	value, present := data[key]
	if !present {
		return !rule.isRequired(data)
	}
	return !rule.ForbiddenWhen.matches(data) && s.validValue(rule, value)
}

func (r auditRule) forData(data map[string]any) auditRule {
	kind, _ := auditText(data[r.ByKey])
	variant, ok := r.Variants[kind]
	if ok {
		variant.Type = r.Type
		return variant
	}
	return r
}

func (r auditRule) isRequired(data map[string]any) bool {
	return r.Required || r.RequiredWhen.matches(data) ||
		(r.RequiredUnless != nil && !r.RequiredUnless.matches(data))
}

func (c *auditCondition) matches(data map[string]any) bool {
	switch {
	case c == nil:
		return false
	case c.Present != "":
		_, ok := data[c.Present]
		return ok
	case c.Absent != "":
		_, ok := data[c.Absent]
		return !ok
	default:
		return c.matchesValue(data[c.Key])
	}
}

func (c *auditCondition) matchesValue(value any) bool {
	switch want := c.Equals.(type) {
	case string:
		actual, ok := auditText(value)
		return ok && actual == want
	case bool:
		v := reflect.ValueOf(value)
		return v.Kind() == reflect.Bool && v.Bool() == want
	default:
		return false
	}
}
