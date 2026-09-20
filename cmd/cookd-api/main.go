// Command cookd-api serves the API behind cookd.dragunov.dev: a cooking coach
// that answers from ingredients and photos, streamed from Claude.
//
// `cookd-api token <email>` prints a session token for that address, signed
// with SESSION_SECRET — the way to call the API from curl without Google.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dragunovartem99/cookd-api/internal/api"
	"github.com/dragunovartem99/cookd-api/internal/auth"
	"github.com/dragunovartem99/cookd-api/internal/chat"
	"github.com/dragunovartem99/cookd-api/internal/config"
	"github.com/dragunovartem99/cookd-api/internal/store"
)

// shutdownGrace is how long in-flight requests have to finish once a signal
// arrives. A streaming answer can be mid-flight, so it is generous.
const shutdownGrace = 30 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	if len(os.Args) > 1 && os.Args[1] == "token" {
		if err := printToken(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if err := run(log); err != nil {
		log.Error("server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	opts := api.Options{
		Store:         db,
		Chat:          chat.NewClaude(cfg.AnthropicKey),
		Signer:        auth.NewSigner(cfg.SessionSecret),
		AdminPassword: cfg.AdminPassword,
		Owner:         cfg.Owner,
		AllowedEmails: cfg.AllowedEmails,
		AllowedOrigin: cfg.AllowedOrigin,
		Logger:        log,
	}
	// Left nil, not wrapped, when off: a nil *Google in the interface would
	// not compare equal to nil and the route would be registered.
	if cfg.GoogleClientID != "" {
		opts.Google = auth.NewGoogle(cfg.GoogleClientID)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.NewServer(opts),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Streaming answers lift this per request; everything else is JSON.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorLog:     slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	// Stop serving on the first SIGINT or SIGTERM; a second one is left to the
	// default handler so an impatient operator can still kill the process.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	listening := make(chan error, 1)
	go func() {
		log.Info("listening", slog.String("addr", cfg.Addr))
		listening <- server.ListenAndServe()
	}()

	select {
	case err := <-listening:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		stop()
		log.Info("shutting down", slog.Duration("grace", shutdownGrace))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

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
