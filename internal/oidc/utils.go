package oidc

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"time"
)

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setCallbackCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

func normalizeIssuer(issuer string) string {
	switch issuer {
	case "accounts.google.com", "https://accounts.google.com":
		return "https://accounts.google.com"
	default:
		return issuer
	}
}
