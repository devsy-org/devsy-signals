package envelope_test

import (
	"os"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
)

func BenchmarkValidateAuditSessionRequest(b *testing.B) {
	raw, err := os.ReadFile("../fixtures/envelope/valid/audit_session_request_ref.json")
	if err != nil {
		b.Fatal(err)
	}
	e, err := envelope.DecodeStrict(raw)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := e.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}
