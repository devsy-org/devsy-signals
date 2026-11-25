// Package envelope defines the Devsy record envelope, level mapping and ULID
// minting. Readers use tolerant Decode; writers use DecodeStrict to self-check.
// Audit ingest uses DecodeStrict plus the audit data allow-list and fails closed.
package envelope

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const maxRecordBytes = 64 << 10

// Decode error categories.
var (
	ErrNotRecord = errors.New("not one JSON record")
	ErrVersion   = errors.New("unsupported record version")
	ErrTooLarge  = errors.New("record too large")
	ErrText      = errors.New("invalid record text")
	ErrStrict    = errors.New("strict decode rejected record")
)

// numberRE is the JSON number grammar. Groups: sign, integer, fraction, exponent sign, exponent.
var numberRE = regexp.MustCompile(`^(-?)(0|[1-9][0-9]*)(?:\.([0-9]+))?(?:[eE]([+-]?)([0-9]+))?$`)

// Decode reads one version 1 JSON record without calling Validate. It ignores
// unknown fields, preserves unknown enum strings, and treats null and values
// that do not fit a typed field as absent. Data and attrs preserve nested nulls.
// Data and attrs keep numbers as json.Number with their original text.
func Decode(data []byte) (Envelope, error) {
	return decode(data, false)
}

// DecodeStrict reads one version 1 JSON record without calling Validate. It
// rejects unknown fields and enum strings, nulls in typed fields, incorrect
// field types, and empty optional strings. Data and attrs preserve nested nulls.
// Data and attrs keep numbers as json.Number with their original text.
func DecodeStrict(data []byte) (Envelope, error) {
	return decode(data, true)
}

func decode(data []byte, strict bool) (Envelope, error) {
	if len(data) > maxRecordBytes {
		return Envelope{}, fmt.Errorf(
			"%w: %d bytes exceeds %d",
			ErrTooLarge,
			len(data),
			maxRecordBytes,
		)
	}
	if err := checkText(data); err != nil {
		return Envelope{}, err
	}
	root, err := parseRecord(data)
	if err != nil {
		return Envelope{}, fmt.Errorf("%w: %w", ErrNotRecord, err)
	}
	if err := checkVersion(root["v"]); err != nil {
		return Envelope{}, err
	}
	delete(root, "v")
	fields := &recordFields{values: root, path: "envelope", strict: strict}
	e := readEnvelope(fields)
	if err := fields.finish(); err != nil {
		return Envelope{}, fmt.Errorf("%w: %w", ErrStrict, err)
	}
	return e, nil
}

func checkVersion(raw any) error {
	switch v := raw.(type) {
	case json.Number:
		if isExactlyOne(string(v)) {
			return nil
		}
		return fmt.Errorf("%w: v must be %d, got %.40s", ErrVersion, Version, v)
	case nil:
		return fmt.Errorf("%w: missing v", ErrVersion)
	default:
		return fmt.Errorf("%w: v must be a number, got %T", ErrVersion, v)
	}
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
