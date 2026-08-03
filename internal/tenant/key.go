package tenant

// CompositeKey builds a tenant-scoped composite key.
// When tenantID is empty, returns sessionKey as-is for backward compatibility.
func CompositeKey(tenantID, sessionKey string) string {
	if tenantID == "" {
		return sessionKey
	}
	return tenantID + ":" + sessionKey
}
