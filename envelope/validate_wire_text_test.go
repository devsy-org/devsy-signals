package envelope_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/devsy-org/devsy-signals/codes"
	"github.com/devsy-org/devsy-signals/envelope"
)

func TestAuditEnvelopeRejectsInvalidWireText(t *testing.T) {
	t.Parallel()
	for name, set := range auditTextFields() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := auditRecord(t, envelope.AuditSessionRequest)
			e.Error = &envelope.Error{
				Code:    "provider.auth_failed",
				Message: "failed",
				Class:   codes.ClassEnvironment,
			}
			set(&e, "text\x00")
			if err := e.Validate(); err == nil {
				t.Error("Validate accepted NUL")
			}
			_, err := envelope.DecodeStrict(mustMarshalJSON(t, e))
			if !errors.Is(err, envelope.ErrText) {
				t.Errorf("DecodeStrict after marshaling NUL: got %v, want ErrText", err)
			}
			set(&e, "text\xff")
			if err := e.Validate(); err == nil {
				t.Error("Validate accepted invalid UTF-8")
			}
		})
	}
}

func auditTextFields() map[string]func(*envelope.Envelope, string) {
	return map[string]func(*envelope.Envelope, string){
		"actor.id":   func(e *envelope.Envelope, s string) { e.Actor.ID = s },
		"actor.kind": func(e *envelope.Envelope, s string) { e.Actor.Kind = envelope.ActorKind(s) },
		"workspace":  func(e *envelope.Envelope, s string) { e.Workspace = s },
		"session":    func(e *envelope.Envelope, s string) { e.Session = s },
		"pool":       func(e *envelope.Envelope, s string) { e.Pool = s },
		"host":       func(e *envelope.Envelope, s string) { e.Host = s },
		"traceId":    func(e *envelope.Envelope, s string) { e.TraceID = s },
		"msg":        func(e *envelope.Envelope, s string) { e.Msg = s },
		"error.code": func(e *envelope.Envelope, s string) { e.Error.Code = s },
		"error.message": func(e *envelope.Envelope, s string) {
			e.Error.Message = s
		},
		"error.hint": func(e *envelope.Envelope, s string) { e.Error.Hint = s },
		"error.context key": func(e *envelope.Envelope, s string) {
			e.Error.Context = map[string]string{s: "value"}
		},
		"error.context value": func(e *envelope.Envelope, s string) {
			e.Error.Context = map[string]string{"name": s}
		},
	}
}

func TestAuditWireRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	e := auditRecord(t, envelope.AuditSessionRequest)
	e.Actor.ID = "wire-text-marker"
	wire := bytes.Replace(mustMarshalJSON(t, e), []byte("wire-text-marker"), []byte("text\xff"), 1)
	if _, err := envelope.DecodeStrict(wire); !errors.Is(err, envelope.ErrText) {
		t.Errorf("DecodeStrict after inserting invalid UTF-8: got %v, want ErrText", err)
	}
}

func TestValidateLogRejectsInvalidWireText(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"text\x00", "text\xff"} {
		e, err := envelope.DecodeStrict([]byte(logWith("")))
		if err != nil {
			t.Fatal(err)
		}
		e.Msg = s
		if err := e.Validate(); err == nil {
			t.Errorf("Validate accepted invalid msg %q", s)
		}
		e.Msg = "message"
		e.Attrs = map[string]any{"text": []string{s}}
		if err := e.Validate(); err == nil {
			t.Errorf("Validate accepted invalid attr string %q", s)
		}
	}
}
