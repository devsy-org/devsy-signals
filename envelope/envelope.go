// Package envelope defines the Devsy record envelope: the Go types, strict
// decoding, validation, level mapping and ULID minting. The JSON Schema in
// schema/envelope.schema.json is the contract; Validate implements the same
// rules.
package envelope

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/devsy-org/devsy-signals/codes"
)

// Version is the envelope major version written to the v field.
const Version = 1

// TimeLayout is the layout of ts: RFC 3339 UTC with milliseconds.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// Kind is the record kind.
type Kind string

// Record kinds.
const (
	KindLog    Kind = "log"
	KindStatus Kind = "status"
	KindResult Kind = "result"
	KindError  Kind = "error"
	KindAudit  Kind = "audit"
)

// ActorKind says who caused an audited action.
type ActorKind string

// Actor kinds.
const (
	ActorUser      ActorKind = "user"
	ActorAgent     ActorKind = "agent"
	ActorAdapter   ActorKind = "adapter"
	ActorLocalUser ActorKind = "local_user"
	ActorSystem    ActorKind = "system"
)

// Actor identifies who caused an audited action.
type Actor struct {
	Kind ActorKind `json:"kind"`
	ID   string    `json:"id"`
}

// Error is the structured error carried by a record.
type Error struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Hint      string            `json:"hint,omitempty"`
	Class     codes.Class       `json:"class"`
	Retryable bool              `json:"retryable"`
	Context   map[string]string `json:"context,omitempty"`
}

// Envelope is one record. Optional string fields are omitted when empty. An
// empty string is never a valid value for any field, so empty means absent.
type Envelope struct {
	V                 int    `json:"v"`
	Kind              Kind   `json:"kind"`
	Type              string `json:"type,omitempty"`
	ID                string `json:"id,omitempty"`
	TS                string `json:"ts"`
	Src               string `json:"src"`
	Level             Level  `json:"level,omitempty"`
	Msg               string `json:"msg,omitempty"`
	TraceID           string `json:"traceId,omitempty"`
	OperationID       string `json:"operationId,omitempty"`
	ParentOperationID string `json:"parentOperationId,omitempty"`
	Workspace         string `json:"workspace,omitempty"`
	Session           string `json:"session,omitempty"`
	Pool              string `json:"pool,omitempty"`
	Host              string `json:"host,omitempty"`

	// PolicyVersion hashes the workspace rules as sha256: plus 64 lowercase hex digits.
	PolicyVersion string `json:"policyVersion,omitempty"`
	// ConfigVersion hashes the whole normalized pool file as sha256: plus 64 lowercase hex digits.
	ConfigVersion string `json:"configVersion,omitempty"`

	Actor *Actor         `json:"actor,omitempty"`
	Error *Error         `json:"error,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Known reports whether k is a known kind.
func (k Kind) Known() bool {
	switch k {
	case KindLog, KindStatus, KindResult, KindError, KindAudit:
		return true
	}
	return false
}

// Known reports whether k is a known actor kind.
func (k ActorKind) Known() bool {
	switch k {
	case ActorUser, ActorAgent, ActorAdapter, ActorLocalUser, ActorSystem:
		return true
	}
	return false
}

// MarshalJSON keeps a non-nil empty data or attrs map on the wire, because
// data is required on some kinds and an empty object satisfies that.
func (e Envelope) MarshalJSON() ([]byte, error) {
	type plain Envelope
	w := struct {
		plain
		Data  *map[string]any `json:"data,omitempty"`
		Attrs *map[string]any `json:"attrs,omitempty"`
	}{plain: plain(e)}
	if e.Data != nil {
		w.Data = &e.Data
	}
	if e.Attrs != nil {
		w.Attrs = &e.Attrs
	}
	out, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	return out, nil
}

// FormatTime formats t as an envelope ts value.
func FormatTime(t time.Time) string {
	return t.UTC().Format(TimeLayout)
}
