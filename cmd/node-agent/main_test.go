package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeStoragePath(t *testing.T) {
	root := t.TempDir()
	got, err := safeStoragePath(root, "objects/file-id/task-id")
	if err != nil {
		t.Fatalf("expected valid storage key: %v", err)
	}
	if !strings.HasPrefix(got, filepath.Clean(root)+string(filepath.Separator)) {
		t.Fatalf("path %q is not under root %q", got, root)
	}
	for _, key := range []string{"../escape", "objects/../../escape", "/absolute/path", `objects\\escape`} {
		if _, err := safeStoragePath(root, key); err == nil {
			t.Errorf("expected storage key %q to be rejected", key)
		}
	}
}

func TestHashFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	sha, size, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len("archive")) {
		t.Fatalf("size = %d, want %d", size, len("archive"))
	}
	if sha != "0eb3e36bfb24dcd9bb1d1bece1531216b59539a8fde17ee80224af0653c92aa3" {
		t.Fatalf("sha256 = %s, unexpected digest", sha)
	}
}
