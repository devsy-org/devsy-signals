package envelope

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Attribute limits.
const (
	MaxAttrs          = 64
	MaxAttrKeyBytes   = 128
	MaxAttrValueBytes = 4096
	maxTypeBytes      = 128
)

var (
	keyRE  = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)
	ulidRE = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
	tsRE   = regexp.MustCompile(
		`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}Z$`,
	)
	traceIDRE = regexp.MustCompile(`^[0-9a-f]{32}$`)
	spanIDRE  = regexp.MustCompile(`^[0-9a-f]{16}$`)
	versionRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	typeRE    = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
)

type problems []error

func (p *problems) add(ok bool, format string, args ...any) {
	if !ok {
		*p = append(*p, fmt.Errorf(format, args...))
	}
}

// Validate applies the envelope schema and audit data rules and joins every
// violation. Strings are measured in bytes where the design sets a byte limit.
// Use maps for nested data objects. Data and attrs reject custom marshalers.
// Data validation charges a lower bound on expanded [Envelope.MarshalJSON]
// bytes. It returns [ErrTooLarge] if those charges exceed 64 KiB. A record
// that fits the wire limit cannot exhaust this budget.
func (e Envelope) Validate() error {
	return errors.Join(
		e.checkHeader(),
		e.checkRequired(),
		e.checkAudit(),
		e.checkIDs(),
		e.checkActor(),
		e.checkError(),
		e.checkAttrs(),
		e.checkVersions(),
		e.checkWireText(),
		e.checkData(),
	)
}

func (e Envelope) checkWireText() error {
	var p problems
	p.wireString("workspace", e.Workspace)
	p.wireString("session", e.Session)
	p.wireString("pool", e.Pool)
	p.wireString("host", e.Host)
	p.wireString("msg", e.Msg)
	return errors.Join(p...)
}

func (p *problems) wireString(field, value string) {
	if !validWireString(value) {
		p.add(false, "%s must be valid UTF-8 without NUL", field)
	}
}

func validWireString(value string) bool {
	return utf8.ValidString(value) && strings.IndexByte(value, 0) < 0
}

func (e Envelope) checkHeader() error {
	var p problems
	p.add(e.V == Version, "v must be %d, got %d", Version, e.V)
	p.add(e.Kind.Known(), "kind %q is not log, status, result, error or audit", e.Kind)
	p.add(validTS(e.TS), "ts %q must be RFC 3339 UTC with milliseconds", e.TS)
	p.add(e.Src.Known(), "src %q is not a known source", e.Src)
	p.add(e.Level == "" || e.Level.Known(), "level %q is not a known level", e.Level)
	p.add(
		e.Type == "" || validRecordType(e.Kind, e.Type),
		"type %q is invalid for kind %s",
		e.Type,
		e.Kind,
	)
	p.add(e.ID == "" || ulidRE.MatchString(e.ID), "id %q is not a ULID", e.ID)
	return errors.Join(p...)
}

func validRecordType(kind Kind, typ string) bool {
	switch kind {
	case KindAudit:
		return slices.Contains(AuditTypes(), typ)
	case KindStatus, KindResult, KindError:
		return len(typ) <= maxTypeBytes && typeRE.MatchString(typ)
	default:
		return validKey(typ, maxTypeBytes)
	}
}

func (e Envelope) checkRequired() error {
	var p problems
	for _, field := range e.missingFields() {
		p.add(false, "%s is required for kind %s", field, e.Kind)
	}
	p.add(e.Kind != KindAudit || !AuditTypeRequiresSession(e.Type) || e.Session != "",
		"session is required for audit type %q", e.Type)
	return errors.Join(p...)
}

type missing []string

func (m *missing) need(field string, ok bool) {
	if !ok {
		*m = append(*m, field)
	}
}

func (m *missing) typeAndID(e Envelope) {
	m.need("type", e.Type != "")
	m.need("id", e.ID != "")
}

// missingFields lists the fields that e.Kind requires and e leaves unset.
func (e Envelope) missingFields() []string {
	var m missing
	switch e.Kind {
	case KindLog:
		m.need("level", e.Level != "")
		m.need("msg", e.Msg != "")
	case KindStatus:
		m.typeAndID(e)
		m.need("level", e.Level != "")
		m.need("data", e.Data != nil)
	case KindResult:
		m.typeAndID(e)
		m.need("data", e.Data != nil)
	case KindError:
		m.typeAndID(e)
		m.need("error", e.Error != nil)
	case KindAudit:
		m.typeAndID(e)
		m.need("data", e.Data != nil)
		m.need("pool", e.Pool != "")
		m.need("host", e.Host != "")
		m.need("policyVersion", e.PolicyVersion != "")
		m.need("configVersion", e.ConfigVersion != "")
		m.need("actor", e.Actor != nil)
	default:
		// checkHeader reports an unknown kind.
	}
	return m
}

// IsVersionHash reports whether s is a sha256: prefix followed by 64 lowercase hex digits.
func IsVersionHash(s string) bool {
	return versionRE.MatchString(s)
}

func (e Envelope) checkVersions() error {
	var p problems
	p.add(
		e.PolicyVersion == "" || IsVersionHash(e.PolicyVersion),
		"policyVersion must be a lowercase SHA-256 hash",
	)
	p.add(
		e.ConfigVersion == "" || IsVersionHash(e.ConfigVersion),
		"configVersion must be a lowercase SHA-256 hash",
	)
	return errors.Join(p...)
}

func (e Envelope) checkAudit() error {
	if e.Kind != KindAudit {
		return nil
	}
	var p problems
	p.add(e.Msg == "", "msg is not allowed on audit")
	p.add(e.Level == "", "level is not allowed on audit")
	p.add(e.Attrs == nil, "attrs is not allowed on audit")
	return errors.Join(errors.Join(p...), e.checkAuditData())
}

func (e Envelope) checkIDs() error {
	var p problems
	p.add(
		validHex(e.TraceID, traceIDRE),
		"traceId %q must be 32 lowercase hex digits and not zero",
		e.TraceID,
	)
	p.add(
		validHex(e.OperationID, spanIDRE),
		"operationId %q must be 16 lowercase hex digits and not zero",
		e.OperationID,
	)
	p.add(validHex(e.ParentOperationID, spanIDRE),
		"parentOperationId %q must be 16 lowercase hex digits and not zero", e.ParentOperationID)
	return errors.Join(p...)
}

func (e Envelope) checkActor() error {
	if e.Actor == nil {
		return nil
	}
	var p problems
	p.add(e.Actor.Kind.Known(), "actor.kind %q is not a known actor kind", e.Actor.Kind)
	p.add(e.Actor.ID != "", "actor.id is required")
	p.wireString("actor.id", e.Actor.ID)
	return errors.Join(p...)
}

func (e Envelope) checkError() error {
	if e.Error == nil {
		return nil
	}
	var p problems
	p.add(keyRE.MatchString(e.Error.Code), "error.code %q is malformed", e.Error.Code)
	p.add(e.Error.Message != "", "error.message is required")
	p.add(e.Error.Class.Known(), "error.class %q is not a known class", e.Error.Class)
	p.wireString("error.message", e.Error.Message)
	p.wireString("error.hint", e.Error.Hint)
	for _, key := range slices.Sorted(maps.Keys(e.Error.Context)) {
		p.wireString("error.context key", key)
		p.wireString("error.context value", e.Error.Context[key])
	}
	return errors.Join(p...)
}

func (e Envelope) checkAttrs() error {
	var p problems
	p.add(len(e.Attrs) <= MaxAttrs, "attrs has %d keys, the limit is %d", len(e.Attrs), MaxAttrs)
	for _, key := range slices.Sorted(maps.Keys(e.Attrs)) {
		p.add(validKey(key, MaxAttrKeyBytes), "attrs key %q is malformed", key)
		if err := attrValueErr(e.Attrs[key]); err != nil {
			p.add(false, "attrs[%q]: %w", key, err)
		}
	}
	return errors.Join(p...)
}

// attrValueErr accepts what marshals to a scalar or to an array of scalars
// that the schema allows. Typed slices and arrays are checked by reflection.
func attrValueErr(v any) error {
	rv := reflect.ValueOf(v)
	if err := wireMarshalerErr(rv); err != nil {
		return err
	}
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			return errors.New("a nil slice marshals to null")
		}
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return errors.New("a byte slice marshals to a base64 string, use a string")
		}
		return elementsErr(rv)
	case reflect.Array:
		return elementsErr(rv)
	default:
		return scalarErr(v)
	}
}

func elementsErr(rv reflect.Value) error {
	var errs []error
	for i := range rv.Len() {
		if err := attrElementErr(rv.Index(i)); err != nil {
			errs = append(errs, fmt.Errorf("element %d: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func attrElementErr(v reflect.Value) error {
	if err := wireMarshalerErr(v); err != nil {
		return err
	}
	if v.Kind() == reflect.Interface {
		if err := wireMarshalerErr(v.Elem()); err != nil {
			return err
		}
	}
	return scalarErr(v.Interface())
}

func scalarErr(v any) error {
	if n, ok := v.(json.Number); ok {
		return numberErr(string(n))
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return stringErr(rv.String())
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return nil
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr:
		return nil
	case reflect.Float32, reflect.Float64:
		return floatErr(rv.Float())
	default:
		return fmt.Errorf("unsupported value type %T", v)
	}
}

func numberErr(n string) error {
	if !numberRE.MatchString(n) {
		return fmt.Errorf("invalid number %q", n)
	}
	return nil
}

func stringErr(s string) error {
	if err := wireStringErr(s); err != nil {
		return err
	}
	if len(s) > MaxAttrValueBytes {
		return fmt.Errorf("string value is %d bytes, the limit is %d", len(s), MaxAttrValueBytes)
	}
	return nil
}

func floatErr(f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return errors.New("NaN and Inf are not JSON numbers")
	}
	return nil
}

func validTS(s string) bool {
	if !tsRE.MatchString(s) {
		return false
	}
	_, err := time.Parse(TimeLayout, s)
	return err == nil
}

func validKey(s string, maxBytes int) bool {
	return len(s) <= maxBytes && keyRE.MatchString(s)
}

func validHex(s string, re *regexp.Regexp) bool {
	return s == "" || (re.MatchString(s) && strings.Trim(s, "0") != "")
}
