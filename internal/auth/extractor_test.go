package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockExtractor struct {
	ac  *AuthContext
	err error
}

func (m *mockExtractor) Extract(_ *http.Request) (*AuthContext, error) {
	return m.ac, m.err
}

func TestExtractorChain(t *testing.T) {
	acFirst := &AuthContext{Identity: "first", AuthMethod: "mock"}
	acSecond := &AuthContext{Identity: "second", AuthMethod: "mock"}
	errFailed := errors.New("extraction failed")

	tests := []struct {
		name       string
		extractors []CredentialExtractor
		wantAC     *AuthContext
		wantErr    bool
	}{
		{
			name: "first match wins",
			extractors: []CredentialExtractor{
				&mockExtractor{ac: acFirst},
				&mockExtractor{ac: acSecond},
			},
			wantAC:  acFirst,
			wantErr: false,
		},
		{
			name: "skip nil results",
			extractors: []CredentialExtractor{
				&mockExtractor{ac: nil, err: nil},
				&mockExtractor{ac: acSecond},
			},
			wantAC:  acSecond,
			wantErr: false,
		},
		{
			name: "all nil returns nil nil",
			extractors: []CredentialExtractor{
				&mockExtractor{ac: nil, err: nil},
				&mockExtractor{ac: nil, err: nil},
			},
			wantAC:  nil,
			wantErr: false,
		},
		{
			name: "error stops chain",
			extractors: []CredentialExtractor{
				&mockExtractor{ac: nil, err: errFailed},
				&mockExtractor{ac: acSecond},
			},
			wantAC:  nil,
			wantErr: true,
		},
		{
			name:       "empty chain returns nil nil",
			extractors: []CredentialExtractor{},
			wantAC:     nil,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := NewExtractorChain(tt.extractors...)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			ac, err := chain.Extract(req)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ac != tt.wantAC {
				t.Errorf("got AuthContext %v, want %v", ac, tt.wantAC)
			}
		})
	}
}
