package schematest_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestRegisteredAuditTypesAgreeWithSchema(t *testing.T) {
	t.Parallel()
	s := compile(t)
	base, err := readFile(filepath.Join(root, "fixtures/envelope/valid/audit_egress_deny.json"))
	require.NoError(t, err)
	for _, typ := range envelope.AuditTypes() {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			var record map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(base, &record))
			wireType, err := json.Marshal(typ)
			require.NoError(t, err)
			record["type"] = wireType
			if !envelope.AuditTypeRequiresSession(typ) {
				delete(record, "session")
			}
			raw, err := json.Marshal(record)
			require.NoError(t, err)
			assert.NoError(t, schemaVerdict(t, s, raw), "schema")
			assert.NoError(t, goVerdict(raw), "Go validator")
		})
	}
}

func TestSchemaAuditTypeEnumMatchesRegistry(t *testing.T) {
	t.Parallel()
	raw, err := readFile(filepath.Join(root, "schema/envelope.schema.json"))
	require.NoError(t, err)
	var doc any
	dec := json.NewDecoder(bytes.NewReader(raw))
	require.NoError(t, dec.Decode(&doc))
	values := at[[]any](t, doc, "$defs", "auditType", "enum")
	want := envelope.AuditTypes()
	got := make([]string, 0, len(values))
	for _, v := range values {
		typ, ok := v.(string)
		require.True(t, ok, "audit type %v is not a string", v)
		got = append(got, typ)
	}
	assert.ElementsMatch(t, want, got)
}
