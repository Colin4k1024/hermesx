package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsCredentialPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory")
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"ssh dir", filepath.Join(home, ".ssh", "id_rsa"), true},
		{"ssh config", filepath.Join(home, ".ssh", "config"), true},
		{"aws credentials", filepath.Join(home, ".aws", "credentials"), true},
		{"aws config", filepath.Join(home, ".aws", "config"), true},
		{"docker config", filepath.Join(home, ".docker", "config.json"), true},
		{"kube config", filepath.Join(home, ".kube", "config"), true},
		{"gh config", filepath.Join(home, ".config", "gh", "hosts.yml"), true},
		{"azure dir", filepath.Join(home, ".azure", "token"), true},
		{"gnupg", filepath.Join(home, ".gnupg", "private-keys-v1.d"), true},
		{"netrc", filepath.Join(home, ".netrc"), true},
		{"env file", filepath.Join(home, ".env"), true},
		{"env.local", filepath.Join(home, ".env.local"), true},
		{"normal file", filepath.Join(home, "Documents", "file.txt"), false},
		{"project code", filepath.Join(home, "projects", "app", "main.go"), false},
		{"outside home", "/tmp/test.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCredentialPath(tt.path)
			if got != tt.want {
				t.Errorf("IsCredentialPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
