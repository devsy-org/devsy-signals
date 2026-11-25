package envelope_test

import (
	"slices"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestAuditTypesMatchRegisteredValues(t *testing.T) {
	t.Parallel()
	cases := []struct{ got, want string }{
		{envelope.AuditSessionRequest, "session.request"},
		{envelope.AuditSessionSpawn, "session.spawn"},
		{envelope.AuditSessionReady, "session.ready"},
		{envelope.AuditSessionStart, "session.start"},
		{envelope.AuditSessionIdle, "session.idle"},
		{envelope.AuditSessionEnd, "session.end"},
		{envelope.AuditSessionReclaim, "session.reclaim"},
		{envelope.AuditPolicyApply, "policy.apply"},
		{envelope.AuditPoolStart, "pool.start"},
		{envelope.AuditCredentialIssue, "credential.issue"},
		{envelope.AuditCredentialUse, "credential.use"},
		{envelope.AuditCredentialDeny, "credential.deny"},
		{envelope.AuditCredentialRevoke, "credential.revoke"},
		{envelope.AuditEgressDeny, "egress.deny"},
		{envelope.AuditEgressAllowSummary, "egress.allow.summary"},
		{envelope.AuditEgressAllow, "egress.allow"},
	}
	want := make([]string, 0, len(cases))
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got constant %q, want %q", c.got, c.want)
		}
		want = append(want, c.want)
	}
	got := envelope.AuditTypes()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("got audit types %q, want %q", got, want)
	}
}

func TestAuditTypeOptions(t *testing.T) {
	t.Parallel()
	var noSession, localOnly []string
	for _, typ := range envelope.AuditTypes() {
		if !envelope.AuditTypeRequiresSession(typ) {
			noSession = append(noSession, typ)
		}
		if envelope.AuditTypeLocalOnly(typ) {
			localOnly = append(localOnly, typ)
		}
	}
	slices.Sort(noSession)
	if want := []string{
		envelope.AuditPolicyApply,
		envelope.AuditPoolStart,
	}; !slices.Equal(
		noSession,
		want,
	) {
		t.Errorf("got no-session types %q, want %q", noSession, want)
	}
	if want := []string{envelope.AuditEgressAllow}; !slices.Equal(localOnly, want) {
		t.Errorf("got local-only types %q, want %q", localOnly, want)
	}
}

func TestAuditTypesReturnsCopy(t *testing.T) {
	t.Parallel()
	types := envelope.AuditTypes()
	if len(types) != 16 {
		t.Fatalf("got %d types, want 16", len(types))
	}
	want := slices.Clone(types)
	types[0] = ""
	if got := envelope.AuditTypes(); !slices.Equal(got, want) {
		t.Fatalf("mutation changed AuditTypes: got %q, want %q", got, want)
	}
}
