package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAuthRequired(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{"/me", "/conversations", "/conversations/x/messages", "/images/1"} {
		if rec := h.do("GET", path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without a token = %d, want 401", path, rec.Code)
		}
	}
	if rec := h.do("GET", "/healthz", "", nil); rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
}

func TestSignedInButNotAllowed(t *testing.T) {
	h := newHarness(t)
	if rec := h.do("GET", "/me", "removed@example.com", nil); rec.Code != http.StatusForbidden {
		t.Errorf("token for an unlisted email = %d, want 403", rec.Code)
	}
}

func TestSignIn(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		credential string
		want       int
	}{
		{"good", http.StatusOK},
		{"stranger", http.StatusForbidden},
		{"forged", http.StatusUnauthorized},
		{"down", http.StatusBadGateway},
		{"", http.StatusBadRequest},
	}
	for _, c := range cases {
		rec := h.do("POST", "/auth/google", "", map[string]string{"credential": c.credential})
		if rec.Code != c.want {
			t.Errorf("credential %q = %d, want %d (%s)", c.credential, rec.Code, c.want, rec.Body)
		}
	}

	rec := h.do("POST", "/auth/google", "", map[string]string{"credential": "good"})
	var out struct{ Token, Email string }
	json.Unmarshal(rec.Body.Bytes(), &out)
	if email, err := h.signer.Verify(out.Token); err != nil || email != me {
		t.Errorf("issued token verifies as %q, %v", email, err)
	}
}

func TestPasswordLogin(t *testing.T) {
	h := newHarness(t)

	for password, want := range map[string]int{password: http.StatusOK, "wrong": http.StatusUnauthorized, "": http.StatusUnauthorized} {
		if rec := h.do("POST", "/auth/login", "", map[string]string{"password": password}); rec.Code != want {
			t.Errorf("password %q = %d, want %d", password, rec.Code, want)
		}
	}

	rec := h.do("POST", "/auth/login", "", map[string]string{"password": password})
	var out struct{ Token, Email string }
	json.Unmarshal(rec.Body.Bytes(), &out)
	if email, err := h.signer.Verify(out.Token); err != nil || email != me || out.Email != me {
		t.Errorf("issued token verifies as %q, %v; want the owner", email, err)
	}
}

func TestPasswordLoginIsRateLimited(t *testing.T) {
	h := newHarness(t)
	var last int
	for range signInRequests + 1 {
		last = h.do("POST", "/auth/login", "", map[string]string{"password": "guess"}).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("attempt %d = %d, want 429", signInRequests+1, last)
	}
}

func TestGoogleSignInCanBeOff(t *testing.T) {
	h := newHarnessWith(t, nil)
	if rec := h.do("POST", "/auth/google", "", map[string]string{"credential": "good"}); rec.Code != http.StatusNotFound {
		t.Errorf("/auth/google with Google off = %d, want 404", rec.Code)
	}
	if rec := h.do("POST", "/auth/login", "", map[string]string{"password": password}); rec.Code != http.StatusOK {
		t.Errorf("/auth/login = %d, want 200", rec.Code)
	}
}
