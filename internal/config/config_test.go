package config

import (
	"strings"
	"testing"
)

func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	env := map[string]string{
		"ALLOWED_ORIGIN":    "https://ui.example",
		"DB_PATH":           "/tmp/cookd.db",
		"ANTHROPIC_API_KEY": "sk-test",
		"GOOGLE_CLIENT_ID":  "client",
		"SESSION_SECRET":    strings.Repeat("s", 32),
		"ALLOWED_EMAILS":    " Me@Example.com, other@example.com ,",
		"PORT":              "50001",
	}
	for k, v := range overrides {
		env[k] = v
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad(t *testing.T) {
	setEnv(t, nil)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":50001" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if _, ok := cfg.AllowedEmails["me@example.com"]; !ok || len(cfg.AllowedEmails) != 2 {
		t.Errorf("AllowedEmails = %v, want two lowercased addresses", cfg.AllowedEmails)
	}
}

func TestLoadPasswordOnly(t *testing.T) {
	setEnv(t, map[string]string{"GOOGLE_CLIENT_ID": "", "ADMIN_PASSWORD": strings.Repeat("p", 20)})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GoogleClientID != "" || cfg.Owner != "me@example.com" {
		t.Errorf("GoogleClientID = %q, Owner = %q; want no Google and the first listed email as owner", cfg.GoogleClientID, cfg.Owner)
	}
}

func TestLoadReportsEveryMissingVariable(t *testing.T) {
	setEnv(t, map[string]string{"ANTHROPIC_API_KEY": "", "DB_PATH": ""})
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") || !strings.Contains(err.Error(), "DB_PATH") {
		t.Fatalf("err = %v, want both missing variables named", err)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	for name, overrides := range map[string]map[string]string{
		"short secret":  {"SESSION_SECRET": "short"},
		"bad port":      {"PORT": "http"},
		"no emails":     {"ALLOWED_EMAILS": " , "},
		"no sign-in":    {"GOOGLE_CLIENT_ID": "", "ADMIN_PASSWORD": ""},
		"weak password": {"ADMIN_PASSWORD": "hunter2"},
	} {
		setEnv(t, overrides)
		if _, err := Load(); err == nil {
			t.Errorf("%s: Load succeeded, want an error", name)
		}
	}
}
