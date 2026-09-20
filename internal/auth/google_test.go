package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const clientID = "client-123.apps.googleusercontent.com"

// fakeGoogle serves a JWKS for a throwaway key and signs tokens with it.
type fakeGoogle struct {
	key     *rsa.PrivateKey
	server  *httptest.Server
	fetches int
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGoogle{key: key}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.fetches++
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGoogle) verifier() *Google {
	g := NewGoogle(clientID)
	g.KeysURL, g.Client = f.server.URL, f.server.Client()
	return g
}

func (f *fakeGoogle) token(t *testing.T, kid string, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	signed := enc(map[string]string{"alg": "RS256", "kid": kid}) + "." + enc(claims)
	digest := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func validClaims() map[string]any {
	return map[string]any{
		"iss": "https://accounts.google.com", "aud": clientID,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"email": "Me@Example.com", "email_verified": true,
	}
}

func TestGoogleVerify(t *testing.T) {
	f := newFakeGoogle(t)
	g := f.verifier()

	email, err := g.Verify(context.Background(), f.token(t, "k1", validClaims()))
	if err != nil || email != "me@example.com" {
		t.Fatalf("Verify = %q, %v", email, err)
	}
}

func TestGoogleVerifyRejects(t *testing.T) {
	f := newFakeGoogle(t)

	mutate := func(key string, value any) map[string]any {
		c := validClaims()
		c[key] = value
		return c
	}
	cases := map[string]string{
		"wrong audience":   f.token(t, "k1", mutate("aud", "someone-else")),
		"wrong issuer":     f.token(t, "k1", mutate("iss", "https://evil.example")),
		"expired":          f.token(t, "k1", mutate("exp", time.Now().Add(-time.Minute).Unix())),
		"unverified email": f.token(t, "k1", mutate("email_verified", false)),
		"unknown key id":   f.token(t, "nope", validClaims()),
		"not a jwt":        "abc",
	}
	// A token signed by a different key than the one Google publishes.
	other := newFakeGoogle(t)
	cases["forged signature"] = other.token(t, "k1", validClaims())

	for name, tok := range cases {
		_, err := f.verifier().Verify(context.Background(), tok)
		if !errors.Is(err, ErrRejected) {
			t.Errorf("%s: err = %v, want ErrRejected", name, err)
		}
	}
}

func TestGoogleCachesKeys(t *testing.T) {
	f := newFakeGoogle(t)
	g := f.verifier()

	for range 3 {
		if _, err := g.Verify(context.Background(), f.token(t, "k1", validClaims())); err != nil {
			t.Fatal(err)
		}
	}
	if f.fetches != 1 {
		t.Errorf("fetched keys %d times, want 1", f.fetches)
	}
}

func TestGoogleUnreachableIsNotARejection(t *testing.T) {
	f := newFakeGoogle(t)
	g := f.verifier()
	f.server.Close()

	_, err := g.Verify(context.Background(), f.token(t, "k1", validClaims()))
	if err == nil || errors.Is(err, ErrRejected) {
		t.Fatalf("err = %v, want a non-rejection failure", err)
	}
}
