// Package tracectx mints and propagates W3C trace context without the
// OpenTelemetry SDK. A Devsy operation is a span: its ID is the parent-id of
// the TRACEPARENT its child processes receive.
package tracectx

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Environment variable names of the OpenTelemetry env carrier.
const (
	EnvTraceparent = "TRACEPARENT"
	EnvTracestate  = "TRACESTATE"
)

const (
	// FlagSampled is the W3C sampled bit.
	FlagSampled = 0x01

	headerLen       = 55
	maxStateBytes   = 512
	maxStateMembers = 32
)

// TraceID is a W3C trace-id.
type TraceID [16]byte

// SpanID is a W3C parent-id, used here as the operation ID.
type SpanID [8]byte

// Context is a parsed traceparent plus its tracestate.
type Context struct {
	TraceID    TraceID
	SpanID     SpanID
	Flags      byte
	TraceState string
}

// NewRoot returns a sampled context with a fresh trace and span.
func NewRoot() Context {
	return Context{TraceID: randomTraceID(), SpanID: randomSpanID(), Flags: FlagSampled}
}

// Child returns a context in the same trace with a fresh span. Flags and
// tracestate carry over.
func Child(parent Context) Context {
	parent.SpanID = randomSpanID()
	return parent
}

// Parse reads a traceparent header value. Version 00 must be exactly 55
// bytes. A later version may append fields after a dash, which are ignored.
func Parse(s string) (Context, error) {
	s = strings.TrimSpace(s)
	if err := checkShape(s); err != nil {
		return Context{}, err
	}
	c, err := decodeFields(s)
	if err != nil {
		return Context{}, err
	}
	if !c.Valid() {
		return Context{}, errors.New("traceparent has an all-zero id")
	}
	return c, nil
}

// String formats c as a version 00 traceparent.
func (c Context) String() string {
	return fmt.Sprintf("00-%s-%s-%02x", c.TraceID, c.SpanID, c.Flags)
}

// Valid reports whether both IDs are non-zero.
func (c Context) Valid() bool {
	return !c.TraceID.IsZero() && !c.SpanID.IsZero()
}

// ToEnv returns environ with TRACEPARENT and TRACESTATE replaced by c. An
// empty tracestate removes the variable.
func (c Context) ToEnv(environ []string) []string {
	out := make([]string, 0, len(environ)+2)
	for _, kv := range environ {
		if name, _, _ := strings.Cut(kv, "="); name != EnvTraceparent && name != EnvTracestate {
			out = append(out, kv)
		}
	}
	out = append(out, EnvTraceparent+"="+c.String())
	if c.TraceState != "" {
		out = append(out, EnvTracestate+"="+c.TraceState)
	}
	return out
}

// FromEnv reads the context from the process environment. It reports false
// when TRACEPARENT is missing or malformed.
func FromEnv() (Context, bool) {
	return FromEnviron(os.Environ())
}

// FromEnviron reads the context from a KEY=VALUE list. The last entry of a
// name wins. A malformed or oversized tracestate is dropped, not an error.
func FromEnviron(environ []string) (Context, bool) {
	var parent, state string
	for _, kv := range environ {
		name, value, _ := strings.Cut(kv, "=")
		switch name {
		case EnvTraceparent:
			parent = value
		case EnvTracestate:
			state = value
		}
	}
	c, err := Parse(parent)
	if err != nil {
		return Context{}, false
	}
	if validState(state) {
		c.TraceState = state
	}
	return c, true
}

// IsZero reports whether t is all zeros.
func (t TraceID) IsZero() bool { return t == TraceID{} }

// String returns 32 lowercase hex digits.
func (t TraceID) String() string { return hex.EncodeToString(t[:]) }

// IsZero reports whether s is all zeros.
func (s SpanID) IsZero() bool { return s == SpanID{} }

// String returns 16 lowercase hex digits.
func (s SpanID) String() string { return hex.EncodeToString(s[:]) }

func checkShape(s string) error {
	if len(s) < headerLen {
		return errors.New("traceparent is too short")
	}
	version, err := hexByte(s[0:2])
	switch {
	case err != nil:
		return fmt.Errorf("traceparent version: %w", err)
	case version == 0xff:
		return errors.New("traceparent version ff is invalid")
	case s[2] != '-' || s[35] != '-' || s[52] != '-':
		return errors.New("traceparent fields must be separated by dashes")
	}
	return checkTrailer(s, version)
}

func checkTrailer(s string, version byte) error {
	switch {
	case version == 0 && len(s) != headerLen:
		return errors.New("traceparent version 00 has trailing data")
	case len(s) > headerLen && s[headerLen] != '-':
		return errors.New("traceparent has trailing data without a dash")
	}
	return nil
}

func decodeFields(s string) (Context, error) {
	var c Context
	if err := decodeHex(c.TraceID[:], s[3:35]); err != nil {
		return Context{}, fmt.Errorf("traceparent trace-id: %w", err)
	}
	if err := decodeHex(c.SpanID[:], s[36:52]); err != nil {
		return Context{}, fmt.Errorf("traceparent parent-id: %w", err)
	}
	flags, err := hexByte(s[53:55])
	if err != nil {
		return Context{}, fmt.Errorf("traceparent flags: %w", err)
	}
	c.Flags = flags
	return c, nil
}

var (
	simpleKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_*/-]{0,255}$`)
	tenantKeyRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_*/-]{0,240}@[a-z][a-z0-9_*/-]{0,13}$`)
	valueRE     = regexp.MustCompile(
		`^[\x20-\x2b\x2d-\x3c\x3e-\x7e]{0,255}[\x21-\x2b\x2d-\x3c\x3e-\x7e]$`,
	)
)

// validState checks the W3C tracestate list grammar: at most 32 list-members
// counting empty ones, unique keys, optional space or tab around commas.
func validState(s string) bool {
	if len(s) == 0 || len(s) > maxStateBytes {
		return false
	}
	members := strings.Split(s, ",")
	if len(members) > maxStateMembers {
		return false
	}
	seen := map[string]bool{}
	for _, member := range members {
		member = strings.Trim(member, " \t")
		if member == "" {
			continue
		}
		key, ok := memberKey(member)
		if !ok || seen[key] {
			return false
		}
		seen[key] = true
	}
	return len(seen) > 0
}

func memberKey(member string) (string, bool) {
	key, value, ok := strings.Cut(member, "=")
	return key, ok && validKey(key) && valueRE.MatchString(value)
}

func validKey(k string) bool {
	return simpleKeyRE.MatchString(k) || tenantKeyRE.MatchString(k)
}

func decodeHex(dst []byte, s string) error {
	if s != strings.ToLower(s) {
		return errors.New("hex must be lowercase")
	}
	if _, err := hex.Decode(dst, []byte(s)); err != nil {
		return fmt.Errorf("decode hex: %w", err)
	}
	return nil
}

func hexByte(s string) (byte, error) {
	var b [1]byte
	err := decodeHex(b[:], s)
	return b[0], err
}

func randomTraceID() TraceID {
	var t TraceID
	for t.IsZero() {
		mustRead(t[:])
	}
	return t
}

func randomSpanID() SpanID {
	var s SpanID
	for s.IsZero() {
		mustRead(s[:])
	}
	return s
}

func mustRead(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic("tracectx: random source failed: " + err.Error())
	}
}
