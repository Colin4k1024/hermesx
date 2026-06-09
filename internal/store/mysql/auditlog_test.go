package mysql

import "testing"

func TestNormalizeAuditArchiveBatchSize(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "default", in: 0, want: 1000},
		{name: "negative default", in: -5, want: 1000},
		{name: "unchanged", in: 250, want: 250},
		{name: "cap", in: 20000, want: 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAuditArchiveBatchSize(tt.in); got != tt.want {
				t.Fatalf("normalizeAuditArchiveBatchSize(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestMySQLPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want string
	}{
		{name: "zero", in: 0, want: ""},
		{name: "one", in: 1, want: "?"},
		{name: "three", in: 3, want: "?,?,?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mysqlPlaceholders(tt.in); got != tt.want {
				t.Fatalf("mysqlPlaceholders(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
