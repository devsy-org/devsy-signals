package envelope_test

import (
	"testing"

	"github.com/devsy-org/devsy-signals/attrkey"
	"github.com/devsy-org/devsy-signals/envelope"
)

type numericJSONName string

func (numericJSONName) MarshalJSON() ([]byte, error) { return []byte("7"), nil }

type emptyTextName string

func (emptyTextName) MarshalText() ([]byte, error) { return []byte{}, nil }

type namedAuditString string

func TestAuditGoAndWireValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value any
		valid bool
	}{
		{name: "builtin text", value: "agent", valid: true},
		{name: "escaped newline", value: "agent\n", valid: true},
		{name: "NUL", value: "agent\x00"},
		{name: "JSON marshaler", value: numericJSONName("agent")},
		{name: "text marshaler", value: emptyTextName("agent")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := auditRecord(t, envelope.AuditSessionRequest)
			e.Data[attrkey.AgentName] = tc.value
			if err := e.Validate(); (err == nil) != tc.valid {
				t.Errorf("Validate before marshal: %v; want valid=%t", err, tc.valid)
			}
			wire, err := e.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			out, verdict := envelope.DecodeStrict(wire)
			if verdict == nil {
				verdict = out.Validate()
			}
			if (verdict == nil) != tc.valid {
				t.Errorf("DecodeStrict then Validate: %v; want valid=%t", verdict, tc.valid)
			}
		})
	}
}

func TestAuditRejectsNamedStringsAndInvalidUTF8(t *testing.T) {
	t.Parallel()
	e := auditRecord(t, envelope.AuditSessionRequest)
	checkAuditCases(t, e, []auditCase{
		{name: "named string", key: attrkey.AgentName, value: namedAuditString("agent")},
		{name: "invalid UTF-8", key: attrkey.AgentName, value: "\xff"},
	})
}
