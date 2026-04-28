package gateway

// GatewaySessionManager is the interface for gateway session management.
// Implemented by SessionStore (local) and PGSessionStore (PostgreSQL).
type GatewaySessionManager interface {
	GetOrCreateSession(source *SessionSource, forceNew bool) *SessionEntry
	ResetSession(sessionKey string) *SessionEntry
	UpdateSession(sessionKey string, lastPromptTokens int)
	SetMemoryFlushed(sessionKey string)
	ListSessions(activeMinutes int) []*SessionEntry
	Close()
}
