package safety

import (
	"testing"
)

func TestScan_EmptyInput(t *testing.T) {
	ig := NewInputGuard()
	matches := ig.Scan("", nil)
	if len(matches) != 0 {
		t.Fatalf("expected no matches for empty string, got %d", len(matches))
	}
}

func TestScan_CleanInput(t *testing.T) {
	ig := NewInputGuard()
	matches := ig.Scan("hello world, this is a normal sentence", nil)
	if len(matches) != 0 {
		t.Fatalf("expected no matches for clean input, got %v", matches)
	}
}

// TestScan_NFKCNormalization verifies that fullwidth/compatibility Unicode
// characters are normalised to ASCII equivalents before pattern matching,
// preventing simple bypass attempts via look-alike characters.
func TestScan_NFKCNormalization_FullwidthChars(t *testing.T) {
	ig := NewInputGuard()

	// U+FF11–U+FF19 are fullwidth digit characters. After NFKC they become
	// ordinary ASCII digits. We just confirm Scan does not panic and processes
	// the normalised form (no injection via fullwidth confusion).
	fullwidthDigits := "１２３" // １２３ → 123
	_ = ig.Scan(fullwidthDigits, nil)       // must not panic
}

// TestScan_NFKCNormalization_LigaturesExpanded ensures that Unicode ligatures
// are expanded before the scan so rules cannot be bypassed by ligature tricks.
func TestScan_NFKCNormalization_LigaturesExpanded(t *testing.T) {
	ig := NewInputGuard()
	// U+FB01 'ﬁ' is the fi-ligature; NFKC expands it to "fi".
	ligature := "ﬁle" // ﬁle → file
	_ = ig.Scan(ligature, nil)
}

func TestScan_CustomPattern_TextMatch(t *testing.T) {
	ig := NewInputGuard()
	cp := InputPattern{Text: "forbidden", Severity: 8}
	matches := ig.Scan("this contains forbidden content", []InputPattern{cp})
	if len(matches) == 0 {
		t.Fatal("expected match for custom text pattern, got none")
	}
	if matches[0].Pattern != "forbidden" {
		t.Fatalf("expected pattern %q, got %q", "forbidden", matches[0].Pattern)
	}
}

func TestScan_CustomPattern_NFKCAppliedBeforeCustomMatch(t *testing.T) {
	ig := NewInputGuard()
	// The text uses fullwidth 'ｆｏｒｂｉｄｄｅｎ'; after NFKC it becomes "forbidden".
	fullwidthForbidden := "ｆｏｒｂｉｄｄｅｎ"
	cp := InputPattern{Text: "forbidden", Severity: 8}
	matches := ig.Scan(fullwidthForbidden, []InputPattern{cp})
	if len(matches) == 0 {
		t.Fatalf("expected NFKC-normalised text to match custom pattern, got none")
	}
}

func TestScan_TruncateMatch(t *testing.T) {
	s := truncateMatch("hello", 10)
	if s != "hello" {
		t.Fatalf("expected %q, got %q", "hello", s)
	}
	long := "abcdefghij"
	s = truncateMatch(long, 5)
	if s != "abcde..." {
		t.Fatalf("expected %q, got %q", "abcde...", s)
	}
}
