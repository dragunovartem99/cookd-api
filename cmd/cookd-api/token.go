package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dragunovartem99/cookd-api/internal/auth"
)

// printToken mints a 24-hour session token for local testing.
func printToken(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: cookd-api token <email>")
	}
	secret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if len(secret) < 32 {
		return errors.New("SESSION_SECRET must be set to at least 32 characters")
	}
	token, _ := auth.NewSigner([]byte(secret)).Issue(strings.ToLower(args[0]), 24*time.Hour)
	fmt.Println(token)
	return nil
}
