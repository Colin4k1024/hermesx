package tools

import "testing"

func TestIsGitArgInjection(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"normal git clone", "git clone https://github.com/user/repo", false},
		{"normal git pull", "git pull origin main", false},
		{"normal git push", "git push -u origin main", false},
		{"upload-pack injection", "git clone --upload-pack=evil https://repo", true},
		{"receive-pack injection", "git push --receive-pack=evil origin", true},
		{"exec injection", "git pull --exec=malicious", true},
		{"core config injection", "git -c core.sshCommand=evil clone repo", true},
		{"credential helper injection", "git -c credential.helper=evil push", true},
		{"http proxy injection", "git -c http.proxy=evil clone repo", true},
		{"ext protocol", "git clone ext::sh -c evil", true},
		{"config http injection", "git --config=http.proxy clone", true},
		{"remote url injection", "git -c remote.origin.url=evil fetch", true},
		{"not a git command", "echo git --upload-pack", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGitArgInjection(tt.command)
			if got != tt.want {
				t.Errorf("IsGitArgInjection(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}
