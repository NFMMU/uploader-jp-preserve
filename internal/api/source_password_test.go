package api

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nfmmu/uploader-archive/internal/config"
)

func TestSourcePasswordEncryptionRoundTrip(t *testing.T) {
	server := &Server{cfg: config.Config{SourcePasswordEncryptionKey: bytes.Repeat([]byte{0x42}, 32)}}
	password := " secret with spaces "
	encrypted, err := server.encryptSourcePassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == nil || strings.Contains(*encrypted, password) {
		t.Fatal("password was not encrypted")
	}
	decrypted, err := server.decryptSourcePassword(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != password {
		t.Fatalf("round trip mismatch: %q", decrypted)
	}
}

func TestSourcePasswordRequiresEncryptionKey(t *testing.T) {
	server := &Server{}
	if _, err := server.encryptSourcePassword("password"); err == nil {
		t.Fatal("expected missing-key error")
	}
	if plain, err := server.decryptSourcePassword(nil); err != nil || plain != "" {
		t.Fatalf("empty password should not need a key: %q %v", plain, err)
	}
}
