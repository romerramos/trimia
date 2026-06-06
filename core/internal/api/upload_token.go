package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type uploadTokenClaims struct {
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Scope    string `json:"scope"`
	Expires  int64  `json:"exp"`
}

func (s *Server) authorizeUpload(r *http.Request) (bool, string) {
	if s.uploadTokenSecret == "" {
		return true, "upload token auth disabled"
	}

	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		return false, "missing bearer token"
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false, "malformed jwt"
	}

	unsignedToken := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(s.uploadTokenSecret))
	_, _ = mac.Write([]byte(unsignedToken))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return false, "invalid jwt signature"
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false, "invalid jwt payload encoding"
	}

	var claims uploadTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return false, "invalid jwt payload json"
	}

	if claims.Subject == "" {
		return false, "missing subject"
	}
	if claims.Audience != "trimia-upload" {
		return false, "invalid audience"
	}
	if claims.Scope != "media:upload" {
		return false, "invalid scope"
	}
	if time.Now().Unix() > claims.Expires {
		return false, "expired token"
	}

	return true, "authorized"
}
