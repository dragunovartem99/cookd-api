package auth

import (
	"strings"
	"testing"
	"time"
)

var secret = []byte(strings.Repeat("s", 32))

func TestSignerRoundTrip(t *testing.T) {
	signer := NewSigner(secret)
	token, _ := signer.Issue("me@example.com", time.Hour)

	email, err := signer.Verify(token)
	if err != nil || email != "me@example.com" {
		t.Fatalf("Verify = %q, %v", email, err)
	}
}

func TestSignerRejects(t *testing.T) {
	signer := NewSigner(secret)
	token, _ := signer.Issue("me@example.com", time.Hour)
	body, _, _ := strings.Cut(token, ".")

	expired := NewSigner(secret)
	expired.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	old, _ := expired.Issue("me@example.com", time.Hour)

	cases := map[string]string{
		"empty":           "",
		"no signature":    body,
		"tampered body":   "x" + token,
		"tampered sig":    token + "x",
		"other secret":    mustIssue(NewSigner([]byte(strings.Repeat("t", 32)))),
		"expired":         old,
		"garbage payload": "!!!.!!!",
	}
	for name, tok := range cases {
		if _, err := signer.Verify(tok); err != ErrInvalidToken {
			t.Errorf("%s: Verify err = %v, want ErrInvalidToken", name, err)
		}
	}
}

func mustIssue(s *Signer) string {
	token, _ := s.Issue("me@example.com", time.Hour)
	return token
}
