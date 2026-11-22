package envelope

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Decode parses one JSON record strictly and does not call Validate. It
// rejects a non-object root, trailing data, duplicate keys at any depth,
// unknown or case-folded names in the envelope, actor and error objects, and
// null for any field that has a value type. Numbers in data and attrs stay
// json.Number, and v may be written as 1.0 or 1e0.
func Decode(data []byte) (Envelope, error) {
	e, err := decode(data)
	if err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	return e, nil
}

func decode(data []byte) (Envelope, error) {
	if err := checkSyntax(data); err != nil {
		return Envelope{}, err
	}
	root, err := rawObject(data, "envelope", isEnvelopeField)
	if err != nil {
		return Envelope{}, err
	}
	changed, vErr := normalizeV(root)
	if err := errors.Join(checkActor(root), checkError(root), vErr); err != nil {
		return Envelope{}, err
	}
	body := data
	if changed {
		if body, err = json.Marshal(root); err != nil {
			return Envelope{}, fmt.Errorf("re-encode record: %w", err)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	var e Envelope
	if err := dec.Decode(&e); err != nil {
		return Envelope{}, fmt.Errorf("decode record: %w", err)
	}
	return e, nil
}

// checkSyntax rejects a non-object root, trailing data and duplicate keys.
func checkSyntax(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if tok, err := nextToken(dec); err != nil || tok != json.Delim('{') {
		return errors.New("the root must be a JSON object")
	}
	if err := scanObject(dec); err != nil {
		return err
	}
	if _, err := nextToken(dec); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON object")
	}
	return nil
}

func nextToken(dec *json.Decoder) (json.Token, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("read JSON token: %w", err)
	}
	return tok, nil
}

// scanObject reads members up to the closing brace of an object whose opening
// brace is already consumed.
func scanObject(dec *json.Decoder) error {
	seen := map[string]bool{}
	for dec.More() {
		tok, err := nextToken(dec)
		if err != nil {
			return err
		}
		key, _ := tok.(string)
		if seen[key] {
			return fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = true
		if err := scanValue(dec); err != nil {
			return err
		}
	}
	_, err := nextToken(dec)
	return err
}

func scanValue(dec *json.Decoder) error {
	tok, err := nextToken(dec)
	if err != nil {
		return err
	}
	switch tok {
	case json.Delim('{'):
		return scanObject(dec)
	case json.Delim('['):
		return scanArray(dec)
	}
	return nil
}

func scanArray(dec *json.Decoder) error {
	for dec.More() {
		if err := scanValue(dec); err != nil {
			return err
		}
	}
	_, err := nextToken(dec)
	return err
}

// rawObject splits a JSON object into raw members. Names must match allowed
// exactly (any name when allowed is nil) and no member may be null.
func rawObject(
	raw []byte,
	what string,
	allowed func(string) bool,
) (map[string]json.RawMessage, error) {
	if isNull(raw) {
		return nil, fmt.Errorf("%s must be an object, not null", what)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	for name, value := range fields {
		if allowed != nil && !allowed(name) {
			return nil, fmt.Errorf("%s has unknown field %q", what, name)
		}
		if isNull(value) {
			return nil, fmt.Errorf("%s.%s is null", what, name)
		}
	}
	return fields, nil
}

func checkActor(root map[string]json.RawMessage) error {
	raw, ok := root["actor"]
	if !ok {
		return nil
	}
	_, err := rawObject(raw, "actor", isActorField)
	return err
}

func checkError(root map[string]json.RawMessage) error {
	raw, ok := root["error"]
	if !ok {
		return nil
	}
	fields, err := rawObject(raw, "error", isErrorField)
	if err != nil {
		return err
	}
	// A bool field cannot tell false from absent.
	if _, ok := fields["retryable"]; !ok {
		return errors.New("error.retryable is required")
	}
	if ctx, ok := fields["context"]; ok {
		_, err = rawObject(ctx, "error.context", nil)
	}
	return err
}

// normalizeV rewrites v to the plain integer 1 when it is exactly 1 in any
// JSON number notation, and rejects every other value. It reports whether the
// text had to be rewritten.
func normalizeV(root map[string]json.RawMessage) (bool, error) {
	raw, ok := root["v"]
	if !ok {
		return false, nil
	}
	text := string(bytes.TrimSpace(raw))
	if !isExactlyOne(text) {
		return false, fmt.Errorf("v must be %d, got %.40s", Version, text)
	}
	if text == "1" {
		return false, nil
	}
	root["v"] = json.RawMessage("1")
	return true, nil
}

// isExactlyOne reports whether a JSON number denotes the value 1. It works on
// the digits and the exponent, so no power of ten is ever built.
func isExactlyOne(text string) bool {
	m := numberRE.FindStringSubmatch(text)
	if m == nil || m[1] == "-" {
		return false
	}
	digits := m[2] + m[3]
	point := len(m[2])
	trimmed := strings.TrimLeft(digits, "0")
	point -= len(digits) - len(trimmed)
	if strings.TrimRight(trimmed, "0") != "1" {
		return false
	}
	exp, ok := exponent(m[4], m[5])
	return ok && point+exp == 1
}

// exponent parses the exponent digits. A magnitude over nine digits cannot
// bring a number back to 1, so it reports false instead of overflowing.
func exponent(sign, digits string) (int, bool) {
	digits = strings.TrimLeft(digits, "0")
	if len(digits) > 9 {
		return 0, false
	}
	n := 0
	for _, c := range digits {
		n = n*10 + int(c-'0')
	}
	if sign == "-" {
		n = -n
	}
	return n, true
}

func isNull(raw []byte) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func isEnvelopeField(name string) bool {
	switch name {
	case "v", "kind", "type", "id", "ts", "src", "level", "msg", "traceId", "operationId",
		"parentOperationId", "workspace", "session", "pool", "host", "policyVersion",
		"actor", "error", "data", "attrs":
		return true
	}
	return false
}

func isActorField(name string) bool {
	return name == "kind" || name == "id"
}

func isErrorField(name string) bool {
	switch name {
	case "code", "message", "hint", "class", "retryable", "context":
		return true
	}
	return false
}
