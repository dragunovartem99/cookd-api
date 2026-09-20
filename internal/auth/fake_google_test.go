package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
