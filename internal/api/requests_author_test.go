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

func TestShouldRejectRequestTargetChange(t *testing.T) {
	tests := []struct {
		name             string
		status           string
		priorAssignments int64
		targetsChanged   bool
		want             bool
	}{
		{name: "unassigned request can change target", status: "in_review", targetsChanged: true, want: false},
		{name: "assigned request cannot change target after reopening", status: "in_review", priorAssignments: 1, targetsChanged: true, want: true},
		{name: "accepted request cannot change target", status: "accepted", targetsChanged: true, want: true},
		{name: "unchanged target is allowed", status: "in_review", priorAssignments: 1, targetsChanged: false, want: false},
		{name: "rejected request without assignment can change target", status: "rejected", targetsChanged: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRejectRequestTargetChange(tt.status, tt.priorAssignments, tt.targetsChanged)
			if got != tt.want {
				t.Fatalf("shouldRejectRequestTargetChange() = %v, want %v", got, tt.want)
			}
		})
	}
}
