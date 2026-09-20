package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	googleKeysURL = "https://www.googleapis.com/oauth2/v3/certs"
	// keysTTL is how long Google's signing keys are trusted without a refetch.
	// Google rotates them every few days, so hours is plenty.
	keysTTL = 6 * time.Hour
	// keysCooldown stops a stream of tokens with unknown key ids from turning
	// into a stream of requests to Google.
	keysCooldown = time.Minute
)

// ErrRejected means the ID token is not one we accept: bad signature, wrong
// audience, expired, or an unverified email. Any other error from Verify is a
// failure to reach Google, not a verdict on the token.
var ErrRejected = errors.New("google id token rejected")

// Google verifies the ID tokens the Google Identity Services button produces.
// It checks the RS256 signature against Google's published keys, then the
// issuer, audience, expiry and email verification.
type Google struct {
	ClientID string
	// KeysURL and Client exist so tests can point the verifier at a fake.
	KeysURL string
	Client  *http.Client

	now func() time.Time

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func NewGoogle(clientID string) *Google {
	return &Google{
		ClientID: clientID,
		KeysURL:  googleKeysURL,
		Client:   &http.Client{Timeout: 10 * time.Second},
		now:      time.Now,
	}
}

// Verify returns the lowercased, verified email an ID token was issued for.
func (g *Google) Verify(ctx context.Context, idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("%w: not a JWT", ErrRejected)
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodePart(parts[0], &header); err != nil || header.Alg != "RS256" {
		return "", fmt.Errorf("%w: unsupported header", ErrRejected)
	}

	key, err := g.key(ctx, header.Kid)
	if err != nil {
		return "", err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("%w: bad signature encoding", ErrRejected)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return "", fmt.Errorf("%w: signature mismatch", ErrRejected)
	}

	var c struct {
		Issuer        string `json:"iss"`
		Audience      string `json:"aud"`
		Expires       int64  `json:"exp"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := decodePart(parts[1], &c); err != nil {
		return "", fmt.Errorf("%w: bad claims", ErrRejected)
	}
	switch {
	case c.Issuer != "https://accounts.google.com" && c.Issuer != "accounts.google.com":
		return "", fmt.Errorf("%w: issuer %q", ErrRejected, c.Issuer)
	case c.Audience != g.ClientID:
		return "", fmt.Errorf("%w: audience mismatch", ErrRejected)
	case g.now().Unix() >= c.Expires:
		return "", fmt.Errorf("%w: expired", ErrRejected)
	case !c.EmailVerified || c.Email == "":
		return "", fmt.Errorf("%w: email not verified", ErrRejected)
	}
	return strings.ToLower(c.Email), nil
}

func decodePart(part string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// key returns the signing key with the given id, refetching Google's key set
// when it is stale or the id is new (a rotation).
func (g *Google) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	fresh := now.Sub(g.fetched) < keysTTL
	if key, ok := g.keys[kid]; ok && fresh {
		return key, nil
	}
	if now.Sub(g.fetched) >= keysCooldown {
		if err := g.refresh(ctx); err != nil {
			return nil, err
		}
	}
	if key, ok := g.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("%w: unknown signing key", ErrRejected)
}

func (g *Google) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.KeysURL, nil)
	if err != nil {
		return err
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch google keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch google keys: status %d", resp.StatusCode)
	}

	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return fmt.Errorf("decode google keys: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	g.keys, g.fetched = keys, g.now()
	return nil
}
