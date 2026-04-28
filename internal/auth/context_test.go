package auth

import (
	"context"
	"testing"
)

func TestWithContextFromContextRoundTrip(t *testing.T) {
	ac := &AuthContext{
		Identity:   "user-1",
		TenantID:   "tenant-1",
		Roles:      []string{"admin"},
		AuthMethod: "static_token",
	}
	ctx := WithContext(context.Background(), ac)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext returned ok=false, want true")
	}
	if got != ac {
		t.Fatalf("FromContext returned %p, want %p", got, ac)
	}
}

func TestFromContextEmptyContext(t *testing.T) {
	got, ok := FromContext(context.Background())
	if ok {
		t.Fatal("FromContext on empty context returned ok=true, want false")
	}
	if got != nil {
		t.Fatalf("FromContext on empty context returned %v, want nil", got)
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		query string
		want  bool
	}{
		{
			name:  "matching role",
			roles: []string{"admin"},
			query: "admin",
			want:  true,
		},
		{
			name:  "non-matching role",
			roles: []string{"admin"},
			query: "user",
			want:  false,
		},
		{
			name:  "multiple roles match first",
			roles: []string{"admin", "operator", "user"},
			query: "admin",
			want:  true,
		},
		{
			name:  "multiple roles match last",
			roles: []string{"admin", "operator", "user"},
			query: "user",
			want:  true,
		},
		{
			name:  "multiple roles no match",
			roles: []string{"admin", "operator"},
			query: "viewer",
			want:  false,
		},
		{
			name:  "empty roles slice",
			roles: []string{},
			query: "admin",
			want:  false,
		},
		{
			name:  "nil roles slice",
			roles: nil,
			query: "admin",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &AuthContext{Roles: tt.roles}
			got := ac.HasRole(tt.query)
			if got != tt.want {
				t.Errorf("HasRole(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
