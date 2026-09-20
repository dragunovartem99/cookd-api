package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

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
