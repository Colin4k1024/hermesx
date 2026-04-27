package tools

import (
	"os"
	"path/filepath"
	"strings"
)

var credentialDirs = []string{
	".docker",
	".azure",
	".config/gh",
	".aws",
	".kube",
	".ssh",
	".gnupg",
}

var credentialFiles = []string{
	".netrc",
	".npmrc",
	".pypirc",
}

// IsCredentialPath checks if a path points to a sensitive credential location.
func IsCredentialPath(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(home, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}

	normalized := filepath.ToSlash(rel)

	for _, dir := range credentialDirs {
		if normalized == dir || strings.HasPrefix(normalized, dir+"/") {
			return true
		}
	}

	for _, file := range credentialFiles {
		if normalized == file {
			return true
		}
	}

	// Block .env files in home directory root
	base := filepath.Base(normalized)
	if !strings.Contains(normalized, "/") {
		if base == ".env" || strings.HasPrefix(base, ".env.") {
			return true
		}
	}

	return false
}
