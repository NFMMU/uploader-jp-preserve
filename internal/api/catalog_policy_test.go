package api

import (
	"context"
	"testing"
)

func TestValidateFileObjectPayloadDefaultsToUnchecked(t *testing.T) {
	s := &Server{}
	policy, err := s.validateFileObjectPayload(context.Background(), &fileObjectPayload{
		OriginalFilename: "sample.bin",
	})
	if err != nil {
		t.Fatalf("validateFileObjectPayload returned an error: %v", err)
	}
	if policy != "unchecked" {
		t.Fatalf("default redistribution policy = %q, want unchecked", policy)
	}
}

func TestRedistributionPoliciesIncludeUncheckedOnly(t *testing.T) {
	if !redistributionPolicies["unchecked"] {
		t.Error("unchecked must be a valid redistribution policy")
	}
	if redistributionPolicies["pending"] {
		t.Error("pending must no longer be a redistribution policy")
	}
	if !redistributionPolicies["unknown"] {
		t.Error("unknown must remain available for a reviewed file with unclear terms")
	}
}
