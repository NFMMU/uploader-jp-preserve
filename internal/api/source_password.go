package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

var errSourcePasswordKeyMissing = errors.New("SOURCE_PASSWORD_ENCRYPTION_KEY is required to use source passwords; set it to the output of openssl rand -hex 32")

func (s *Server) encryptSourcePassword(password string) (*string, error) {
	if password == "" {
		return nil, nil
	}
	if len(s.cfg.SourcePasswordEncryptionKey) != 32 {
		return nil, errSourcePasswordKeyMissing
	}
	block, err := aes.NewCipher(s.cfg.SourcePasswordEncryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(password), nil)
	encoded := base64.RawStdEncoding.EncodeToString(ciphertext)
	return &encoded, nil
}

func (s *Server) decryptSourcePassword(encrypted *string) (string, error) {
	if encrypted == nil || strings.TrimSpace(*encrypted) == "" {
		return "", nil
	}
	if len(s.cfg.SourcePasswordEncryptionKey) != 32 {
		return "", errSourcePasswordKeyMissing
	}
	payload, err := base64.RawStdEncoding.DecodeString(*encrypted)
	if err != nil {
		return "", errors.New("stored source password is unreadable")
	}
	block, err := aes.NewCipher(s.cfg.SourcePasswordEncryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("stored source password is unreadable")
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("stored source password could not be decrypted")
	}
	return string(plain), nil
}
