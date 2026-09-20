// Package config reads the process configuration from the environment.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// minSecretLength keeps the session key out of guessable territory: an HMAC key
// shorter than the hash output adds nothing, so 32 bytes is the floor.
const minSecretLength = 32

// minPasswordLength is a floor, not a recommendation: the endpoint is on the
// public internet, so use a long random string.
const minPasswordLength = 12

// Config is everything the server needs to start. See .env.example.
type Config struct {
	// Addr is the TCP address the HTTP server listens on.
	Addr string
	// AllowedOrigin is the single browser origin CORS lets through.
	AllowedOrigin string
	// DBPath is the SQLite file conversations are kept in.
	DBPath string
	// AnthropicKey authenticates calls to the Claude API.
	AnthropicKey string
	// GoogleClientID is the OAuth client the sign-in button is issued for; an ID
	// token minted for any other audience is refused. Optional: blank turns
	// Google sign-in off.
	GoogleClientID string
	// AdminPassword signs in the owner without Google. Optional: blank turns
	// password sign-in off. At least one of the two must be set.
	AdminPassword string
	// Owner is the account a password sign-in acts as: the first address in
	// ALLOWED_EMAILS.
	Owner string
	// SessionSecret signs the tokens the API hands out after a Google sign-in.
	SessionSecret []byte
	// AllowedEmails are the Google accounts that may sign in, lowercased.
	AllowedEmails map[string]struct{}
}

// Load reads the environment, reporting every missing variable at once so a
// misconfigured deploy takes one restart to diagnose rather than seven.
func Load() (Config, error) {
	var missing []string
	required := func(key string) string {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			missing = append(missing, key)
		}
		return value
	}

	cfg := Config{
		AllowedOrigin:  required("ALLOWED_ORIGIN"),
		DBPath:         required("DB_PATH"),
		AnthropicKey:   required("ANTHROPIC_API_KEY"),
		GoogleClientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
		AllowedEmails:  make(map[string]struct{}),
	}
	secret := required("SESSION_SECRET")
	emails := required("ALLOWED_EMAILS")
	port := required("PORT")

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return Config{}, fmt.Errorf("PORT must be a port number, got %q", port)
	}
	if len(secret) < minSecretLength {
		return Config{}, fmt.Errorf("SESSION_SECRET must be at least %d characters", minSecretLength)
	}
	for _, email := range strings.Split(emails, ",") {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
			if cfg.Owner == "" {
				cfg.Owner = email
			}
			cfg.AllowedEmails[email] = struct{}{}
		}
	}
	if cfg.GoogleClientID == "" && cfg.AdminPassword == "" {
		return Config{}, fmt.Errorf("set GOOGLE_CLIENT_ID or ADMIN_PASSWORD: there would be no way to sign in")
	}
	if cfg.AdminPassword != "" && len(cfg.AdminPassword) < minPasswordLength {
		return Config{}, fmt.Errorf("ADMIN_PASSWORD must be at least %d characters", minPasswordLength)
	}
	if len(cfg.AllowedEmails) == 0 {
		return Config{}, fmt.Errorf("ALLOWED_EMAILS lists no addresses")
	}

	cfg.SessionSecret = []byte(secret)
	cfg.Addr = net.JoinHostPort("", port)

	return cfg, nil
}
