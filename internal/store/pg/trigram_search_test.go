package pg

import "testing"

func TestContainsCJK(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello world", false},
		{"some search query", false},
		{"", false},
		{"123 abc", false},
		{"你好世界", true},
		{"hello 你好", true},
		{"日本語テスト", true},
		{"한국어", true},
		{"カタカナ", true},
		{"mixed English 中文 text", true},
		{"café résumé", false},
		{"über straße", false},
	}

	for _, tt := range tests {
		got := containsCJK(tt.input)
		if got != tt.want {
			t.Errorf("containsCJK(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsCJK(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'中', true},
		{'a', false},
		{'あ', true},
		{'ア', true},
		{'한', true},
		{'1', false},
		{'é', false},
	}

	for _, tt := range tests {
		got := isCJK(tt.r)
		if got != tt.want {
			t.Errorf("isCJK(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}
