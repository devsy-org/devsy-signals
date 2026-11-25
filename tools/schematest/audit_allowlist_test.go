package schematest_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/attrkey"
	"github.com/devsy-org/devsy-signals/envelope"
)

func TestAuditAllowlistMatchesConstants(t *testing.T) {
	t.Parallel()
	raw, err := readFile(filepath.Join(root, "schema/audit-allowlist.json"))
	require.NoError(t, err)
	var spec struct {
		Types map[string]auditEventRule `json:"types"`
	}
	require.NoError(t, json.Unmarshal(raw, &spec))
	want := auditKeySets()
	gotTypes := make([]string, 0, len(spec.Types))
	wantTypes := make([]string, 0, len(want))
	for typ := range spec.Types {
		gotTypes = append(gotTypes, typ)
	}
	for typ, keys := range want {
		wantTypes = append(wantTypes, typ)
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			assertAuditEventRule(t, typ, spec.Types[typ], keys)
		})
	}
	assert.ElementsMatch(t, envelope.AuditTypes(), gotTypes)
	assert.ElementsMatch(t, envelope.AuditTypes(), wantTypes)
}

type auditEventRule struct {
	Keys      map[string]json.RawMessage `json:"keys"`
	NoSession bool                       `json:"noSession"`
	LocalOnly bool                       `json:"localOnly"`
}

func assertAuditEventRule(t *testing.T, typ string, got auditEventRule, want []string) {
	t.Helper()
	keys := make([]string, 0, len(got.Keys))
	for key := range got.Keys {
		keys = append(keys, key)
	}
	assert.ElementsMatch(t, want, keys)
	assert.Equal(t, !envelope.AuditTypeRequiresSession(typ), got.NoSession)
	assert.Equal(t, envelope.AuditTypeLocalOnly(typ), got.LocalOnly)
}

func auditKeySets() map[string][]string {
	return map[string][]string{
		envelope.AuditSessionRequest: {
			attrkey.RequestIDHash, attrkey.RequestOutcome, attrkey.AgentName, attrkey.RequesterKind,
			attrkey.UserID, attrkey.VCSRepositoryName, attrkey.VCSRefHeadName,
		},
		envelope.AuditSessionSpawn: {attrkey.ProviderName, attrkey.ContainerImageID},
		envelope.AuditSessionReady: {
			attrkey.ProviderName,
			attrkey.ContainerImageID,
			attrkey.DurationMS,
		},
		envelope.AuditSessionStart: {attrkey.ProcessExecutableName, attrkey.JobTimeoutMS},
		envelope.AuditSessionIdle:  {attrkey.IdleSince},
		envelope.AuditSessionEnd: {
			attrkey.SessionEndReason, attrkey.ProcessExitCode, attrkey.DurationMS,
		},
		envelope.AuditSessionReclaim: {
			attrkey.ReclaimResult,
			attrkey.ReclaimRemaining,
			attrkey.ReclaimAttempts,
			attrkey.ReclaimOrphaned,
		},
		envelope.AuditPolicyApply: {
			attrkey.PolicyAllowRuleCount, attrkey.PolicyGrantNames, attrkey.PolicyVerifyResult,
		},
		envelope.AuditPoolStart: {
			attrkey.PoolSessionsLoaded,
			attrkey.PoolSessionsFailed,
			attrkey.PoolOrphansFound,
		},
		envelope.AuditCredentialIssue: {
			attrkey.CredentialGrant,
			attrkey.CredentialMode,
			attrkey.CredentialIssuer,
			attrkey.CredentialPermissions,
			attrkey.CredentialRepositoryCount,
			attrkey.CredentialTTLMS,
			attrkey.CredentialExpiresAt,
			attrkey.CredentialFingerprint,
		},
		envelope.AuditCredentialUse: {
			attrkey.CredentialGrant,
			attrkey.RouteName,
			attrkey.HTTPRequestMethod,
			attrkey.HTTPResponseStatusCode,
			attrkey.HTTPRequestBodySize,
			attrkey.HTTPResponseBodySize,
		},
		envelope.AuditCredentialDeny: {
			attrkey.CredentialDenyReason,
			attrkey.CredentialGrant,
			attrkey.RouteName,
		},
		envelope.AuditCredentialRevoke: {
			attrkey.CredentialGrant,
			attrkey.CredentialFingerprint,
			attrkey.CredentialRevokeResult,
		},
		envelope.AuditEgressDeny: {
			attrkey.ServerAddress, attrkey.EgressAddressRedacted, attrkey.ServerPort,
			attrkey.NetworkPeerAddress, attrkey.EgressReason,
		},
		envelope.AuditEgressAllow: {
			attrkey.ServerAddress, attrkey.EgressAddressRedacted, attrkey.ServerPort,
			attrkey.NetworkPeerAddress, attrkey.EgressRuleIndex,
		},
		envelope.AuditEgressAllowSummary: {
			attrkey.ServerAddress,
			attrkey.EgressAddressRedacted,
			attrkey.ServerPort,
			attrkey.EgressCount,
			attrkey.EgressFirstSeen,
			attrkey.EgressLastSeen,
			attrkey.EgressRuleIndex,
		},
	}
}
