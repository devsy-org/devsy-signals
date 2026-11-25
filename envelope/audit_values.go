package envelope

import (
	"encoding/json"
	"math"
	"net/netip"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (s auditSpec) validValue(rule auditRule, value any) bool {
	v := reflect.ValueOf(value)
	defaults := s.Defaults[rule.Type]
	switch rule.Type {
	case "array":
		return s.validArray(rule, v)
	case "boolean":
		return v.Kind() == reflect.Bool && (rule.Const == nil || v.Bool() == *rule.Const)
	case "integer":
		return validAuditInteger(rule, defaults, value)
	case "string":
		return validAuditString(rule, defaults, value)
	default:
		return false
	}
}

func (s auditSpec) validArray(rule auditRule, v reflect.Value) bool {
	if !isAuditArray(v) || v.Len() > auditLimit(rule.MaxItems, s.Defaults["array"].MaxItems) ||
		rule.Items == nil {
		return false
	}
	for i := range v.Len() {
		if !s.validValue(*rule.Items, v.Index(i).Interface()) {
			return false
		}
	}
	return true
}

func isAuditArray(v reflect.Value) bool {
	return v.Kind() == reflect.Array ||
		(v.Kind() == reflect.Slice && !v.IsNil() && v.Type().Elem().Kind() != reflect.Uint8)
}

func auditLimit(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func validAuditString(rule, defaults auditRule, value any) bool {
	text, ok := auditText(value)
	return ok && validAuditText(rule, defaults, text) && matchesAuditFormat(rule.Format, text)
}

func auditText(value any) (string, bool) {
	v := reflect.ValueOf(value)
	if _, numeric := value.(json.Number); numeric || v.Kind() != reflect.String {
		return "", false
	}
	return v.String(), true
}

func validAuditText(rule, defaults auditRule, text string) bool {
	return utf8.ValidString(text) && len(text) >= auditLimit(rule.MinBytes, defaults.MinBytes) &&
		len(text) <= auditLimit(rule.MaxBytes, defaults.MaxBytes) &&
		(rule.Enum == nil || slices.Contains(rule.Enum, text)) && matchesAuditPattern(rule.Pattern, text)
}

func matchesAuditPattern(pattern, text string) bool {
	if pattern == "" {
		return true
	}
	ok, err := regexp.MatchString(pattern, text)
	return err == nil && ok
}

func matchesAuditFormat(format, text string) bool {
	switch format {
	case "":
		return true
	case "basename":
		return validAuditBasename(text)
	case "owner-name":
		return validAuditRepository(text)
	case "git-ref-name":
		return validAuditRef(text)
	case "envelope-timestamp":
		return validTS(text)
	case "sha256-digest":
		return IsVersionHash(text)
	default:
		return matchesAuditAddress(format, text)
	}
}

func validAuditBasename(text string) bool {
	return !strings.ContainsAny(text, "/\\\x00") && text != "." && text != ".."
}

func validAuditRepository(text string) bool {
	owner, name, ok := strings.Cut(text, "/")
	return ok && owner != "" && name != "" && !strings.ContainsAny(text, "\\:@?#") &&
		!strings.Contains(name, "/") && strings.IndexFunc(text, forbiddenRepositoryRune) < 0
}

func forbiddenRepositoryRune(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r)
}

func matchesAuditAddress(format, text string) bool {
	addr, err := netip.ParseAddr(text)
	ip := err == nil && addr.Zone() == ""
	switch format {
	case "ip-literal":
		return ip
	case "lowercase-dns-or-ip-literal":
		return ip || validAuditDNS(text)
	default:
		return false
	}
}

var auditDNSLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func validAuditDNS(name string) bool {
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if !auditDNSLabel.MatchString(label) {
			return false
		}
	}
	return true
}

var auditRefForbidden = regexp.MustCompile(`[\x00-\x20\x7f~^:?*\[\\]|\.\.|@\{|//|^@$|^/|/$|\.$`)

func validAuditRef(ref string) bool {
	if ref == "" || auditRefForbidden.MatchString(ref) {
		return false
	}
	for _, component := range strings.Split(ref, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func validAuditInteger(rule, defaults auditRule, value any) bool {
	n, ok := auditInteger(value)
	if !ok || (rule.Maximum != nil && n > float64(*rule.Maximum)) {
		return false
	}
	minimum := rule.Minimum
	if minimum == nil && !rule.AllowNegative {
		minimum = defaults.Minimum
	}
	return minimum == nil || n >= float64(*minimum)
}

func auditInteger(value any) (float64, bool) {
	if number, ok := value.(json.Number); ok {
		return auditJSONInteger(string(number))
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		n := v.Float()
		return n, !math.IsInf(n, 0) && math.Trunc(n) == n
	default:
		return 0, false
	}
}

func auditJSONInteger(text string) (float64, bool) {
	m := numberRE.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	digits := strings.TrimRight(m[2]+m[3], "0")
	if digits == "" {
		return 0, true
	}
	exp, ok := exponent(m[4], m[5])
	if !ok {
		return math.Copysign(math.Inf(1), auditSign(m[1])), m[4] != "-"
	}
	n, _ := strconv.ParseFloat(text, 64)
	return n, len(m[2])+exp >= len(digits)
}

func auditSign(sign string) float64 {
	if sign == "-" {
		return -1
	}
	return 1
}
