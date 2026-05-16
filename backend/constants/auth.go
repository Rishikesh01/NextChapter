package constants

// Token kinds. Mirrors the CHECK constraint on auth_tokens.kind. Values
// are persisted in the DB column, so the *string content* of these
// constants is part of the schema contract — change them only as part
// of a migration that rewrites existing rows.
const (
	TokenKindSession = "session"
	TokenKindAPI     = "api"
)

// Token prefixes per ADR-0001. A leaked token is greppable by prefix.
// As with the kinds, these are persisted in the plaintext token shown
// to the user; changing them would invalidate every existing token.
const (
	TokenPrefixSession = "ncs_"
	TokenPrefixAPI     = "nca_"
)

// SessionCookieName is the cookie name set by /auth/login and
// /auth/register and consumed by the auth middleware.
const SessionCookieName = "nc_session"
