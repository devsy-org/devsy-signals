package schematest_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/attrkey"
	"github.com/devsy-org/devsy-signals/envelope"
)

func TestSchemaAuditGitRef(t *testing.T) {
	t.Parallel()
	s := compile(t)
	raw, err := readFile(
		filepath.Join(root, "fixtures/envelope/valid/audit_session_request_ref.json"),
	)
	require.NoError(t, err)
	record, err := envelope.DecodeStrict(raw)
	require.NoError(t, err)
	for _, ref := range []string{
		"", "a\x1fb", "a\x7fb", "a b", "a~b", "a^b", "a:b", "a?b", "a*b", "a[b", `a\b`,
		"a..b", "a@{b", "@", "/a", "a/", "a//b", "a/.b", "a.lock/b", "a/b.lock", "a/b.",
		strings.Repeat("a", 257),
	} {
		record.Data[attrkey.VCSRefHeadName] = ref
		raw, err := json.Marshal(record)
		require.NoError(t, err)
		assert.Error(t, schemaVerdict(t, s, raw), "ref %q", ref)
	}
}

func TestRegisteredAuditTypesAgreeWithSchema(t *testing.T) {
	t.Parallel()
	s := compile(t)
	base, err := readFile(filepath.Join(root, "fixtures/envelope/valid/audit_egress_deny.json"))
	require.NoError(t, err)
	examples := auditDataExamples()
	for _, typ := range envelope.AuditTypes() {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			var record map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(base, &record))
			wireType, err := json.Marshal(typ)
			require.NoError(t, err)
			record["type"] = wireType
			data, ok := examples[typ]
			require.True(t, ok, "no audit example for %s", typ)
			record["data"] = json.RawMessage(data)
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

func auditDataExamples() map[string]string {
	return map[string]string{
		envelope.AuditSessionRequest: `{
			"devsy.request.id_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"devsy.request.outcome":"new","devsy.agent.name":"cursor","devsy.requester.kind":"adapter",
			"vcs.repository.name":"owner/name"
		}`,
		envelope.AuditSessionSpawn: `{"devsy.provider.name":"docker"}`,
		envelope.AuditSessionReady: `{
			"devsy.provider.name":"docker","devsy.duration_ms":1,
			"container.image.id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}`,
		envelope.AuditSessionStart: `{"process.executable.name":"agent","devsy.job.timeout_ms":1}`,
		envelope.AuditSessionIdle:  `{"devsy.idle_since":"2026-11-22T10:15:03.123Z"}`,
		envelope.AuditSessionEnd:   `{"devsy.session.end_reason":"completed","devsy.duration_ms":1}`,
		envelope.AuditSessionReclaim: `{
			"devsy.reclaim.result":"deleted","devsy.reclaim.attempts":1,"devsy.reclaim.orphaned":false
		}`,
		envelope.AuditPolicyApply: `{
			"devsy.policy.allow_rule_count":0,"devsy.policy.grant_names":[],"devsy.policy.verify_result":"ok"
		}`,
		envelope.AuditPoolStart: `{
			"devsy.pool.sessions_loaded":0,"devsy.pool.sessions_failed":0,"devsy.pool.orphans_found":0
		}`,
		envelope.AuditCredentialIssue: `{
			"devsy.credential.grant":"source","devsy.credential.mode":"inject","devsy.credential.permissions":[],
			"devsy.credential.repository_count":1,"devsy.credential.ttl_ms":1,
			"devsy.credential.expires_at":"2026-11-22T10:15:03.123Z","devsy.credential.fingerprint":"aaaaaaaaaaaa"
		}`,
		envelope.AuditCredentialUse: `{
			"devsy.credential.grant":"source","devsy.route.name":"upstream","http.request.method":"GET",
			"http.request.body.size":0,"http.response.body.size":0
		}`,
		envelope.AuditCredentialDeny: `{"devsy.credential.deny_reason":"not_granted"}`,
		envelope.AuditCredentialRevoke: `{
			"devsy.credential.grant":"source","devsy.credential.fingerprint":"aaaaaaaaaaaa",
			"devsy.credential.revoke_result":"revoked"
		}`,
		envelope.AuditEgressDeny:  `{"server.address":"example.test","server.port":443,"devsy.egress.reason":"not_allowed"}`,
		envelope.AuditEgressAllow: `{"server.address":"example.test","server.port":443,"network.peer.address":"192.0.2.1"}`,
		envelope.AuditEgressAllowSummary: `{
			"server.address":"example.test","server.port":443,"devsy.egress.count":1,
			"devsy.egress.first_seen":"2026-11-22T10:15:03.123Z","devsy.egress.last_seen":"2026-11-22T10:15:03.123Z"
		}`,
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
