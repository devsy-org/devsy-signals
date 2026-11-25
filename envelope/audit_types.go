package envelope

// Session lifecycle audit types.
const (
	AuditSessionRequest = "session.request"
	AuditSessionSpawn   = "session.spawn"
	AuditSessionReady   = "session.ready"
	AuditSessionStart   = "session.start"
	AuditSessionIdle    = "session.idle"
	AuditSessionEnd     = "session.end"
	AuditSessionReclaim = "session.reclaim"
)

// Pool and policy audit types.
const (
	AuditPolicyApply = "policy.apply"
	AuditPoolStart   = "pool.start"
)

// Credential audit types.
//
//nolint:gosec // These wire labels are event names, not credentials (G101).
const (
	AuditCredentialIssue  = "credential.issue"
	AuditCredentialUse    = "credential.use"
	AuditCredentialDeny   = "credential.deny"
	AuditCredentialRevoke = "credential.revoke"
)

// Egress audit types.
const (
	AuditEgressDeny         = "egress.deny"
	AuditEgressAllowSummary = "egress.allow.summary"
	AuditEgressAllow        = "egress.allow"
)

// AuditTypes returns a fresh slice of all registered audit event types.
func AuditTypes() []string {
	return []string{
		AuditSessionRequest,
		AuditSessionSpawn,
		AuditSessionReady,
		AuditSessionStart,
		AuditSessionIdle,
		AuditSessionEnd,
		AuditSessionReclaim,
		AuditPolicyApply,
		AuditPoolStart,
		AuditCredentialIssue,
		AuditCredentialUse,
		AuditCredentialDeny,
		AuditCredentialRevoke,
		AuditEgressDeny,
		AuditEgressAllowSummary,
		AuditEgressAllow,
	}
}

// AuditTypeRequiresSession reports whether an audit type needs a session ID.
// It does not check whether the type is registered.
func AuditTypeRequiresSession(typ string) bool {
	return typ != AuditPolicyApply && typ != AuditPoolStart
}

// AuditTypeLocalOnly reports whether an audit type must stay local.
func AuditTypeLocalOnly(typ string) bool {
	return typ == AuditEgressAllow
}
