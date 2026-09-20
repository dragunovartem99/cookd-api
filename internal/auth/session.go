// Package auth proves who is calling: Google's ID tokens on the way in, the
// API's own signed session tokens afterwards.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrInvalidToken is returned for any session token that is malformed, forged
// or expired; callers deliberately cannot tell which.
var ErrInvalidToken = errors.New("invalid session token")

// Signer issues and checks session tokens: base64url(payload) "." base64url(HMAC-SHA256).
// The token carries the email and an expiry, so the server keeps no session
// state — revoking someone means dropping them from ALLOWED_EMAILS, which is
// re-checked on every request.
type Signer struct {
	secret []byte
	now    func() time.Time
}

func NewSigner(secret []byte) *Signer {
	return &Signer{secret: secret, now: time.Now}
}

type claims struct {
	Email   string `json:"e"`
	Expires int64  `json:"x"`
}

// Issue signs a token for email that stays valid for ttl.
func (s *Signer) Issue(email string, ttl time.Duration) (string, time.Time) {
	expires := s.now().Add(ttl).Truncate(time.Second)
	payload, _ := json.Marshal(claims{Email: email, Expires: expires.Unix()})
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + s.sign(body), expires
}

// Verify returns the email a token was issued to.
func (s *Signer) Verify(token string) (string, error) {
	body, signature, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(s.sign(body))) {
		return "", ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", ErrInvalidToken
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil || c.Email == "" || s.now().Unix() >= c.Expires {
		return "", ErrInvalidToken
	}
	return c.Email, nil
}

func (s *Signer) sign(body string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
