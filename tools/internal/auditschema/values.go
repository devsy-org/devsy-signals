package auditschema

import (
	"fmt"
	"strings"
)

func (s spec) value(f field) (node, error) {
	defaults := s.Defaults[f.Type]
	switch f.Type {
	case "string":
		return text(f, defaults)
	case "integer":
		return integer(f, defaults), nil
	case "array":
		return s.array(f, defaults)
	case "boolean":
		out := node{typeKey: f.Type}
		if f.Const != nil {
			out["const"] = *f.Const
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported audit value type %q", f.Type)
	}
}

func (s spec) array(f, defaults field) (node, error) {
	if f.Items == nil {
		return nil, fmt.Errorf("array rule has no item rule")
	}
	items, err := s.value(*f.Items)
	if err != nil {
		return nil, err
	}
	return node{
		typeKey:    f.Type,
		"items":    items,
		"maxItems": limit(f.MaxItems, defaults.MaxItems),
	}, nil
}

func integer(f, defaults field) node {
	out := node{typeKey: f.Type}
	minimum := f.Minimum
	if minimum == nil && !f.AllowNegative {
		minimum = defaults.Minimum
	}
	if minimum != nil {
		out["minimum"] = *minimum
	}
	if f.Maximum != nil {
		out["maximum"] = *f.Maximum
	}
	return out
}

func text(f, defaults field) (node, error) {
	out := node{
		typeKey:     f.Type,
		"minLength": limit(f.MinBytes, defaults.MinBytes),
		"maxLength": limit(f.MaxBytes, defaults.MaxBytes),
	}
	if f.Enum != nil {
		out["enum"] = f.Enum
	}
	if f.Pattern != "" {
		out[patternKey] = f.Pattern
	}
	if f.Format != "" {
		format, err := textFormat(f.Format)
		if err != nil {
			return nil, err
		}
		out["allOf"] = []node{format}
	}
	return out, nil
}

func limit(override, fallback int) int {
	if override > 0 {
		return override
	}
	return fallback
}

func textFormat(format string) (node, error) {
	switch format {
	case "basename":
		return negate(node{patternKey: `[/\\\x00]|^\.\.?$`}), nil
	case "owner-name":
		const segment = `[^/\\:@?#\x00-\x20\x7f-\x9f\p{Z}]+`
		return node{patternKey: `^` + segment + `/` + segment + `$`}, nil
	case "git-ref-name":
		return node{refKey: "envelope.schema.json#/$defs/auditGitRef"}, nil
	case "envelope-timestamp":
		return node{refKey: "envelope.schema.json#/properties/ts"}, nil
	case "sha256-digest":
		return node{refKey: "envelope.schema.json#/$defs/versionHash"}, nil
	default:
		return addressFormat(format)
	}
}

func addressFormat(format string) (node, error) {
	forms := []node{
		{patternKey: "^" + ipv4Pattern + "$"},
		{patternKey: ipv6Pattern()},
	}
	switch format {
	case "ip-literal":
	case "lowercase-dns-or-ip-literal":
		const label = `[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?`
		forms = append(forms, node{patternKey: `^` + label + `(\.` + label + `)*\.?$`})
	default:
		return nil, fmt.Errorf("unsupported audit format %q", format)
	}
	return node{"anyOf": forms}, nil
}

const (
	octet       = `(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])`
	ipv4Pattern = `(?:` + octet + `\.){3}` + octet
	hextet      = `[0-9a-fA-F]{1,4}`
)

func ipv6Pattern() string {
	forms := []string{`(?:` + hextet + `:){7}` + hextet, `(?:` + hextet + `:){6}` + ipv4Pattern}
	forms = append(forms, compressedIPv6(8, "")...)
	forms = append(forms, compressedIPv6(6, ipv4Pattern)...)
	return "^(?:" + strings.Join(forms, "|") + ")$"
}

func compressedIPv6(groups int, tail string) []string {
	forms := make([]string, 0, groups)
	for left := range groups {
		prefix := fmt.Sprintf(`(?:%s:){%d}:`, hextet, left)
		if left == 0 {
			prefix = "::"
		}
		forms = append(forms, prefix+compressedRight(groups-left-1, tail))
	}
	return forms
}

func compressedRight(maximum int, tail string) string {
	if tail != "" {
		return fmt.Sprintf(`(?:%s:){0,%d}%s`, hextet, maximum, tail)
	}
	if maximum == 0 {
		return ""
	}
	return fmt.Sprintf(`(?:%s(?::%s){0,%d})?`, hextet, hextet, maximum-1)
}
