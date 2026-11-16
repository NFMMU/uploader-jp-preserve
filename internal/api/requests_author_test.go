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

func TestSameOptionalRequestTarget(t *testing.T) {
	id := "artifact-id"
	otherID := "other-artifact-id"

	tests := []struct {
		name string
		left *string
		right *string
		want bool
	}{
		{name: "both nil", want: true},
		{name: "same value", left: &id, right: &id, want: true},
		{name: "different values", left: &id, right: &otherID, want: false},
		{name: "left nil", right: &id, want: false},
		{name: "right nil", left: &id, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameOptionalRequestTarget(tt.left, tt.right); got != tt.want {
				t.Fatalf("sameOptionalRequestTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}
