package auth

import "testing"

func TestHasScope_LegacyEmptyScopes_NonAdmin(t *testing.T) {
	ac := &AuthContext{Scopes: []string{}}
	// Legacy keys with empty scopes get non-admin access.
	if !ac.HasScope("read") {
		t.Error("HasScope(\"read\") with empty Scopes should return true")
	}
	if !ac.HasScope("write") {
		t.Error("HasScope(\"write\") with empty Scopes should return true")
	}
}

func TestHasScope_LegacyEmptyScopes_Admin(t *testing.T) {
	ac := &AuthContext{Scopes: []string{}}
	// Admin scope always requires explicit grant.
	if ac.HasScope("admin") {
		t.Error("HasScope(\"admin\") with empty Scopes should return false")
	}
}

func TestHasScope_LegacyNilScopes_NonAdmin(t *testing.T) {
	ac := &AuthContext{Scopes: nil}
	if !ac.HasScope("read") {
		t.Error("HasScope(\"read\") with nil Scopes should return true")
	}
}

func TestHasScope_LegacyNilScopes_Admin(t *testing.T) {
	ac := &AuthContext{Scopes: nil}
	if ac.HasScope("admin") {
		t.Error("HasScope(\"admin\") with nil Scopes should return false")
	}
}

func TestHasScope_ExplicitScopes_Match(t *testing.T) {
	ac := &AuthContext{Scopes: []string{"read", "write"}}
	if !ac.HasScope("read") {
		t.Error("HasScope(\"read\") should return true when in explicit Scopes")
	}
	if !ac.HasScope("write") {
		t.Error("HasScope(\"write\") should return true when in explicit Scopes")
	}
}

func TestHasScope_ExplicitScopes_NoMatch(t *testing.T) {
	ac := &AuthContext{Scopes: []string{"read"}}
	if ac.HasScope("admin") {
		t.Error("HasScope(\"admin\") should return false when not in explicit Scopes")
	}
	if ac.HasScope("write") {
		t.Error("HasScope(\"write\") should return false when not in explicit Scopes")
	}
}

func TestHasScope_ExplicitAdminScope(t *testing.T) {
	ac := &AuthContext{Scopes: []string{"admin", "read"}}
	if !ac.HasScope("admin") {
		t.Error("HasScope(\"admin\") should return true when explicitly granted")
	}
	if !ac.HasScope("read") {
		t.Error("HasScope(\"read\") should return true when explicitly granted")
	}
}

func TestHasScope_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		query  string
		want   bool
	}{
		{"legacy_read", nil, "read", true},
		{"legacy_admin", nil, "admin", false},
		{"empty_write", []string{}, "write", true},
		{"empty_admin", []string{}, "admin", false},
		{"explicit_match", []string{"read", "write"}, "write", true},
		{"explicit_miss", []string{"read", "write"}, "delete", false},
		{"explicit_admin_granted", []string{"admin"}, "admin", true},
		{"explicit_non_admin_miss", []string{"admin"}, "read", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &AuthContext{Scopes: tt.scopes}
			got := ac.HasScope(tt.query)
			if got != tt.want {
				t.Errorf("HasScope(%q) with scopes=%v: got %v, want %v",
					tt.query, tt.scopes, got, tt.want)
			}
		})
	}
}
