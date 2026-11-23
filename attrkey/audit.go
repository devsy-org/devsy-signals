package attrkey

// Keys used in the data object of pool audit records. Each audit type allows
// a fixed subset of these keys and of the OpenTelemetry keys above.
const (
	RequestIDHash         = "devsy.request.id_hash"
	RequestOutcome        = "devsy.request.outcome"
	AgentName             = "devsy.agent.name"
	RequesterKind         = "devsy.requester.kind"
	JobTimeoutMS          = "devsy.job.timeout_ms"
	IdleSince             = "devsy.idle_since"
	SessionEndReason      = "devsy.session.end_reason"
	ReclaimResult         = "devsy.reclaim.result"
	ReclaimRemaining      = "devsy.reclaim.remaining"
	ReclaimAttempts       = "devsy.reclaim.attempts"
	ReclaimOrphaned       = "devsy.reclaim.orphaned"
	PolicyAllowRuleCount  = "devsy.policy.allow_rule_count"
	PolicyGrantNames      = "devsy.policy.grant_names"
	PolicyVerifyResult    = "devsy.policy.verify_result"
	PoolSessionsLoaded    = "devsy.pool.sessions_loaded"
	PoolSessionsFailed    = "devsy.pool.sessions_failed"
	PoolOrphansFound      = "devsy.pool.orphans_found"
	RouteName             = "devsy.route.name"
	EgressReason          = "devsy.egress.reason"
	EgressAddressRedacted = "devsy.egress.address_redacted"
	EgressRuleIndex       = "devsy.egress.rule_index"
	EgressCount           = "devsy.egress.count"
	EgressFirstSeen       = "devsy.egress.first_seen"
	EgressLastSeen        = "devsy.egress.last_seen"
)

// Credential audit keys. Values are names, counts, times and fingerprints,
// never secrets.
//
//nolint:gosec // G101 matches the identifier names; these are key names.
const (
	CredentialGrant           = "devsy.credential.grant"
	CredentialMode            = "devsy.credential.mode"
	CredentialIssuer          = "devsy.credential.issuer"
	CredentialPermissions     = "devsy.credential.permissions"
	CredentialRepositoryCount = "devsy.credential.repository_count"
	CredentialTTLMS           = "devsy.credential.ttl_ms"
	CredentialExpiresAt       = "devsy.credential.expires_at"
	CredentialFingerprint     = "devsy.credential.fingerprint"
	CredentialDenyReason      = "devsy.credential.deny_reason"
	CredentialRevokeResult    = "devsy.credential.revoke_result"
)
