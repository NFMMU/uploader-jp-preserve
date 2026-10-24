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

func TestCleanupUnverifiedObject(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "objects", "file-id", "task-id")
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		destination: "stale-corrupt-object",
		filepath.Join(filepath.Dir(destination), ".partial-old-a"): "partial-a",
		filepath.Join(filepath.Dir(destination), ".partial-old-b"): "partial-b",
		filepath.Join(filepath.Dir(destination), "keep.txt"):       "unrelated",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanupUnverifiedObject(destination); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{destination, filepath.Join(filepath.Dir(destination), ".partial-old-a"), filepath.Join(filepath.Dir(destination), ".partial-old-b")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected %q to be removed, stat error = %v", path, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(destination), "keep.txt")); err != nil || string(b) != "unrelated" {
		t.Errorf("unrelated file was not preserved: content=%q err=%v", string(b), err)
	}
}
