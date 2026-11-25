package envelope_test

import (
	"encoding/json"
	"maps"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-signals/attrkey"
	"github.com/devsy-org/devsy-signals/envelope"
)

const otherAuditValue = "other"

func auditRecord(t *testing.T, typ string) envelope.Envelope {
	t.Helper()
	e, err := decodeFile(
		t,
		filepath.Join("..", "fixtures", "envelope", "valid", "audit_session_request_ref.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if typ != e.Type {
		e.Type, e.Data = typ, map[string]any{}
	}
	return e
}

type auditCase struct {
	name  string
	key   string
	value any
	valid bool
}

func checkAuditCases(t *testing.T, base envelope.Envelope, cases []auditCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := base
			e.Data = maps.Clone(base.Data)
			if tc.value == nil {
				delete(e.Data, tc.key)
			} else {
				e.Data[tc.key] = tc.value
			}
			err := e.Validate()
			if (err == nil) != tc.valid || (err != nil && !strings.Contains(err.Error(), tc.key)) {
				t.Errorf("Validate() = %v; want valid=%t for %s", err, tc.valid, tc.key)
			}
		})
	}
}

func TestValidateAuditSessionRequest(t *testing.T) {
	t.Parallel()
	e := auditRecord(t, envelope.AuditSessionRequest)
	checkAuditCases(t, e, []auditCase{
		{name: "unknown key", key: "unexpected", value: true},
		{name: "required key", key: attrkey.AgentName},
		{name: "empty string", key: attrkey.AgentName, value: ""},
		{name: "string byte limit", key: attrkey.AgentName, value: strings.Repeat("\u00e9", 129)},
		{name: "request hash", key: attrkey.RequestIDHash, value: strings.Repeat("A", 64)},
		{name: "outcome enum", key: attrkey.RequestOutcome, value: otherAuditValue},
		{
			name:  "repo name",
			key:   attrkey.VCSRepositoryName,
			value: "https://example.com/owner/name",
		},
		{name: "login required", key: attrkey.UserID},
		{name: "login empty", key: attrkey.UserID, value: ""},
		{
			name:  "login 64 bytes",
			key:   attrkey.UserID,
			value: strings.Repeat("\u00e9", 32),
			valid: true,
		},
		{name: "login 66 bytes", key: attrkey.UserID, value: strings.Repeat("\u00e9", 33)},
		{name: "login invalid UTF-8", key: attrkey.UserID, value: "\xff"},
		{name: "login C0", key: attrkey.UserID, value: "a\x1fb"},
		{name: "login DEL", key: attrkey.UserID, value: "a\x7fb"},
		{name: "login C1", key: attrkey.UserID, value: "a\u0081b"},
		{name: "login LS", key: attrkey.UserID, value: "a\u2028b"},
		{name: "login PS", key: attrkey.UserID, value: "a\u2029b"},
		{name: "ref missing", key: attrkey.VCSRefHeadName, valid: true},
	})
}

func TestValidateAuditAdapter(t *testing.T) {
	t.Parallel()
	e := auditRecord(t, envelope.AuditSessionRequest)
	e.Data[attrkey.RequesterKind] = "adapter"
	checkAuditCases(t, e, []auditCase{
		{name: "optional", key: attrkey.UserID, valid: true},
		{name: "claude", key: attrkey.UserID, value: "claude:9A_-", valid: true},
		{
			name:  "cursor",
			key:   attrkey.UserID,
			value: "cursor:" + strings.Repeat("a", 128),
			valid: true,
		},
		{name: "provider", key: attrkey.UserID, value: "other:123"},
		{name: "empty ID", key: attrkey.UserID, value: "claude:"},
		{name: "ID too long", key: attrkey.UserID, value: "cursor:" + strings.Repeat("a", 129)},
		{name: "email", key: attrkey.UserID, value: "claude:a@example.com"},
	})
}

func TestValidateAuditCredential(t *testing.T) {
	t.Parallel()
	e := auditRecord(t, envelope.AuditCredentialIssue)
	e.Data = map[string]any{
		attrkey.CredentialGrant:           "example",
		attrkey.CredentialMode:            "mint",
		attrkey.CredentialIssuer:          "github-app",
		attrkey.CredentialPermissions:     []string{"contents:read", "checks:write"},
		attrkey.CredentialRepositoryCount: 0,
		attrkey.CredentialTTLMS:           1000,
		attrkey.CredentialExpiresAt:       e.TS,
		attrkey.CredentialFingerprint:     "012345abcdef",
	}
	many := strings.Fields(strings.Repeat("contents:read ", 65))
	checkAuditCases(t, e, []auditCase{
		{name: "missing mint issuer", key: attrkey.CredentialIssuer},
		{name: "issuer enum", key: attrkey.CredentialIssuer, value: otherAuditValue},
		{name: "permissions required", key: attrkey.CredentialPermissions},
		{
			name:  "empty permissions",
			key:   attrkey.CredentialPermissions,
			value: []string{},
			valid: true,
		},
		{name: "null permissions", key: attrkey.CredentialPermissions, value: []string(nil)},
		{
			name:  "permission count 64",
			key:   attrkey.CredentialPermissions,
			value: many[:64],
			valid: true,
		},
		{name: "permission count 65", key: attrkey.CredentialPermissions, value: many},
		{
			name:  "admin not allowed",
			key:   attrkey.CredentialPermissions,
			value: []any{"contents:admin"},
		},
		{name: "permission element type", key: attrkey.CredentialPermissions, value: []any{1}},
		{name: "timestamp", key: attrkey.CredentialExpiresAt, value: "2026-02-30T10:15:03.123Z"},
		{name: "fingerprint", key: attrkey.CredentialFingerprint, value: "012345ABCDEF"},
		{
			name:  "integer exponent",
			key:   attrkey.CredentialRepositoryCount,
			value: json.Number("2.0e1"),
			valid: true,
		},
		{
			name:  "integer fraction",
			key:   attrkey.CredentialRepositoryCount,
			value: json.Number("1.0000000000000000001"),
		},
		{name: "integer type", key: attrkey.CredentialTTLMS, value: "10"},
		{name: "integer nonfinite", key: attrkey.CredentialTTLMS, value: math.Inf(1)},
	})
	e.Data = maps.Clone(e.Data)
	e.Data[attrkey.CredentialMode] = "inject"
	checkAuditCases(t, e, []auditCase{
		{name: "inject forbids issuer", key: attrkey.CredentialIssuer, value: "github-app"},
		{name: "inject without issuer", key: attrkey.CredentialIssuer, valid: true},
	})
}

func TestValidateAuditEgress(t *testing.T) {
	t.Parallel()
	e := auditRecord(t, envelope.AuditEgressAllow)
	e.Data = map[string]any{
		attrkey.ServerAddress:      "example.test",
		attrkey.ServerPort:         443,
		attrkey.NetworkPeerAddress: "192.0.2.1",
	}
	checkAuditCases(t, e, []auditCase{
		{name: "local only allowed", key: attrkey.ServerPort, value: 65535, valid: true},
		{name: "port lower bound", key: attrkey.ServerPort, value: 0},
		{name: "port upper bound", key: attrkey.ServerPort, value: 65536},
		{name: "IP literal valid", key: attrkey.ServerAddress, value: "2001:db8::1", valid: true},
		{name: "DNS case", key: attrkey.ServerAddress, value: "Example.test"},
		{name: "DNS label", key: attrkey.ServerAddress, value: "-example.test"},
		{name: "reject URL", key: attrkey.ServerAddress, value: "https://example.test/path"},
		{name: "peer must be IP", key: attrkey.NetworkPeerAddress, value: "example.test"},
		{name: "missing address", key: attrkey.ServerAddress},
		{name: "redacted address present", key: attrkey.EgressAddressRedacted, value: true},
	})
	e.Data = maps.Clone(e.Data)
	delete(e.Data, attrkey.ServerAddress)
	checkAuditCases(t, e, []auditCase{
		{name: "redacted", key: attrkey.EgressAddressRedacted, value: true, valid: true},
		{name: "redacted only true", key: attrkey.EgressAddressRedacted, value: false},
		{name: "redacted wrong type", key: attrkey.EgressAddressRedacted, value: "true"},
	})
}

func TestValidateAuditOtherFormats(t *testing.T) {
	t.Parallel()
	e := auditRecord(t, envelope.AuditSessionStart)
	e.Data[attrkey.JobTimeoutMS] = 0
	checkAuditCases(t, e, []auditCase{
		{name: "basename", key: attrkey.ProcessExecutableName, value: "runner", valid: true},
		{name: "not path", key: attrkey.ProcessExecutableName, value: "/bin/runner"},
	})
	e = auditRecord(t, envelope.AuditSessionSpawn)
	e.Data[attrkey.ProviderName] = "example"
	checkAuditCases(t, e, []auditCase{
		{
			name:  "image digest",
			key:   attrkey.ContainerImageID,
			value: "sha256:" + strings.Repeat("a", 64),
			valid: true,
		},
		{name: "not image tag", key: attrkey.ContainerImageID, value: "runner:latest"},
	})
	e = auditRecord(t, envelope.AuditSessionReclaim)
	e.Data = map[string]any{
		attrkey.ReclaimAttempts: 1,
		attrkey.ReclaimResult:   "deleted",
		attrkey.ReclaimOrphaned: false,
	}
	checkAuditCases(t, e, []auditCase{
		{name: "attempts minimum", key: attrkey.ReclaimAttempts, value: 0},
		{
			name:  "remaining limit",
			key:   attrkey.ReclaimRemaining,
			value: strings.Fields(strings.Repeat("volume ", 33)),
		},
		{name: "remaining enum", key: attrkey.ReclaimRemaining, value: []string{otherAuditValue}},
	})
	e = auditRecord(t, envelope.AuditSessionEnd)
	e.Data = map[string]any{attrkey.SessionEndReason: "failed", attrkey.DurationMS: 0}
	checkAuditCases(t, e, []auditCase{
		{name: "negative exit code", key: attrkey.ProcessExitCode, value: -1, valid: true},
		{name: "negative duration", key: attrkey.DurationMS, value: -1},
	})
}
