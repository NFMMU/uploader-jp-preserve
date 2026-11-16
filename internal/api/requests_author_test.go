package api

import (
	"strings"
	"testing"
)

func TestLimitAuthorDisplayName(t *testing.T) {
	longName := strings.Repeat("あ", 121)
	got := limitAuthorDisplayName(longName)
	if len([]rune(got)) != 120 {
		t.Fatalf("expected 120 runes, got %d", len([]rune(got)))
	}

	if got := limitAuthorDisplayName("  Author  "); got != "Author" {
		t.Fatalf("expected trimmed name, got %q", got)
	}
}
