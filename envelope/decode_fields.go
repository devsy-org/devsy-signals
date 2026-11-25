package envelope

import (
	"fmt"
	"regexp"

	"github.com/devsy-org/devsy-signals/codes"
)

// Source identifies where a record originated.
type Source string

var sourceRE = regexp.MustCompile(
	`^(cli|desktop|agent|controlplane|pool|provider/[a-z0-9][a-z0-9._-]*)$`,
)

// Known reports whether s is a known source or a well-formed provider source.
func (s Source) Known() bool {
	return sourceRE.MatchString(string(s))
}

type recordFields struct {
	values map[string]any
	path   string
	strict bool
	err    error
}

func readEnvelope(f *recordFields) Envelope {
	return Envelope{
		V:                 Version,
		Kind:              readEnum[Kind](f, "kind"),
		Type:              readOptionalString(f, "type"),
		ID:                readOptionalString(f, "id"),
		TS:                readField[string](f, "ts"),
		Src:               readEnum[Source](f, "src"),
		Level:             readEnum[Level](f, "level"),
		Msg:               readOptionalString(f, "msg"),
		TraceID:           readOptionalString(f, "traceId"),
		OperationID:       readOptionalString(f, "operationId"),
		ParentOperationID: readOptionalString(f, "parentOperationId"),
		Workspace:         readOptionalString(f, "workspace"),
		Session:           readOptionalString(f, "session"),
		Pool:              readOptionalString(f, "pool"),
		Host:              readOptionalString(f, "host"),
		PolicyVersion:     readOptionalString(f, "policyVersion"),
		Actor:             readObject(f, "actor", readActor),
		Error:             readObject(f, "error", readError),
		Data:              readField[map[string]any](f, "data"),
		Attrs:             readField[map[string]any](f, "attrs"),
	}
}

func readActor(f *recordFields) *Actor {
	return &Actor{Kind: readEnum[ActorKind](f, "kind"), ID: readField[string](f, "id")}
}

func readError(f *recordFields) *Error {
	if _, ok := f.values["retryable"]; f.strict && !ok {
		f.fail("retryable", "is required")
	}
	return &Error{
		Code:      readField[string](f, "code"),
		Message:   readField[string](f, "message"),
		Hint:      readOptionalString(f, "hint"),
		Class:     readEnum[codes.Class](f, "class"),
		Retryable: readField[bool](f, "retryable"),
		Context:   readObject(f, "context", readContext),
	}
}

func readContext(f *recordFields) map[string]string {
	values := make(map[string]string, len(f.values))
	for name, value := range f.values {
		if s, ok := value.(string); ok {
			values[name] = s
		} else if f.strict {
			f.fail(name, "must be a string")
		}
		delete(f.values, name)
	}
	return values
}

func readObject[T any](f *recordFields, name string, read func(*recordFields) T) T {
	values := readField[map[string]any](f, name)
	if values == nil {
		var zero T
		return zero
	}
	nested := &recordFields{values: values, path: f.path + "." + name, strict: f.strict}
	out := read(nested)
	if err := nested.finish(); f.err == nil {
		f.err = err
	}
	return out
}

func readField[T any](f *recordFields, name string) T {
	raw, present := f.values[name]
	delete(f.values, name)
	value, ok := raw.(T)
	if present && !ok && f.strict {
		if raw == nil {
			f.fail(name, "is null")
		} else {
			f.fail(name, fmt.Sprintf("has the wrong type (%T)", raw))
		}
	}
	return value
}

func readOptionalString(f *recordFields, name string) string {
	_, present := f.values[name]
	s := readField[string](f, name)
	if f.strict && present && s == "" {
		f.fail(name, "must not be empty")
	}
	return s
}

type knownString interface {
	~string
	Known() bool
}

func readEnum[T knownString](f *recordFields, name string) T {
	_, present := f.values[name]
	value := T(readField[string](f, name))
	if f.strict && present && !value.Known() {
		f.fail(name, fmt.Sprintf("has unknown value %q", value))
	}
	return value
}

func (f *recordFields) fail(name, reason string) {
	if f.err == nil {
		f.err = fmt.Errorf("%s.%s %s", f.path, name, reason)
	}
}

func (f *recordFields) finish() error {
	if f.strict && f.err == nil {
		for name := range f.values {
			return fmt.Errorf("%s has unknown field %q", f.path, name)
		}
	}
	return f.err
}
