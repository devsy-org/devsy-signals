package schematest_test

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/attrkey"
	"github.com/devsy-org/devsy-signals/envelope"
)

type auditMutation struct {
	name string
	set  map[string]any
	drop string
}

func TestAuditDataMutationFixtures(t *testing.T) {
	t.Parallel()
	s := compile(t)
	raw, err := readFile(filepath.Join(root, "fixtures/envelope/valid/audit_egress_deny.json"))
	require.NoError(t, err)
	base, err := envelope.DecodeStrict(raw)
	require.NoError(t, err)
	for typ, example := range auditDataExamples() {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			e := base
			e.Type, e.Data = typ, nil
			if !envelope.AuditTypeRequiresSession(typ) {
				e.Session = ""
			}
			require.NoError(t, json.Unmarshal([]byte(example), &e.Data))
			checkAuditMutations(t, s, e)
		})
	}
}

func checkAuditMutations(t *testing.T, s *jsonschema.Schema, e envelope.Envelope) {
	t.Helper()
	keys := slices.Sorted(maps.Keys(e.Data))
	require.NotEmpty(t, keys)
	key := keys[0]
	cases := []auditMutation{
		{name: "unknown key", set: map[string]any{"unexpected": true}},
		{name: "missing required key", drop: key},
		{name: "wrong type", set: map[string]any{key: []int{1}}},
		oversizeMutation(keys, e.Data),
	}
	for _, tc := range append(cases, conditionalMutations(e.Type)...) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			record := e
			record.Data = maps.Clone(e.Data)
			delete(record.Data, tc.drop)
			maps.Copy(record.Data, tc.set)
			raw, err := json.Marshal(record)
			require.NoError(t, err)
			require.Error(t, goVerdict(raw), "Go")
			assert.Error(t, schemaVerdict(t, s, raw), "schema")
		})
	}
}

func oversizeMutation(keys []string, data map[string]any) auditMutation {
	for _, key := range keys {
		switch data[key].(type) {
		case string:
			return auditMutation{
				name: "oversize string",
				set:  map[string]any{key: strings.Repeat("x", 257)},
			}
		case []any:
			return auditMutation{
				name: "oversize array",
				set:  map[string]any{key: slices.Repeat([]string{"x"}, 65)},
			}
		}
	}
	// pool.start has only nonnegative integers, with no upper limit.
	return auditMutation{name: "out of range", set: map[string]any{keys[0]: -1}}
}

func conditionalMutations(typ string) []auditMutation {
	switch typ {
	case envelope.AuditSessionRequest:
		return []auditMutation{
			{
				name: "local user requires ID",
				set:  map[string]any{attrkey.RequesterKind: "local_user"},
			},
			{name: "invalid adapter ID", set: map[string]any{attrkey.UserID: "cursor:"}},
		}
	case envelope.AuditCredentialIssue:
		return []auditMutation{
			{name: "mint requires issuer", set: map[string]any{attrkey.CredentialMode: "mint"}},
			{
				name: "inject forbids issuer",
				set:  map[string]any{attrkey.CredentialIssuer: "github-app"},
			},
			{
				name: "admin permission",
				set:  map[string]any{attrkey.CredentialPermissions: []string{"contents:admin"}},
			},
		}
	case envelope.AuditEgressDeny, envelope.AuditEgressAllow, envelope.AuditEgressAllowSummary:
		return []auditMutation{
			{name: "missing address and redaction", drop: attrkey.ServerAddress},
			{
				name: "redaction forbids address",
				set:  map[string]any{attrkey.EgressAddressRedacted: true},
			},
			{
				name: "redaction must be true",
				drop: attrkey.ServerAddress,
				set:  map[string]any{attrkey.EgressAddressRedacted: false},
			},
			{name: "port above range", set: map[string]any{attrkey.ServerPort: 65536}},
			{
				name: "invalid address",
				set:  map[string]any{attrkey.ServerAddress: "http://example.com"},
			},
		}
	default:
		return nil
	}
}
