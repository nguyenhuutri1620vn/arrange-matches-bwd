// Command server runs the badminton court manager: JSON API plus the
// embedded web app, backed by one SQLite file.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"arrange-matches-bwd/internal/finance"
	"arrange-matches-bwd/internal/httpapi"
	"arrange-matches-bwd/internal/session"
	"arrange-matches-bwd/internal/storage"
	"arrange-matches-bwd/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := loadDotEnv(".env"); err != nil {
		return err
	}
	username, password := os.Getenv("ADMIN_USERNAME"), os.Getenv("ADMIN_PASSWORD")
	if username == "" || password == "" {
		return errors.New("ADMIN_USERNAME and ADMIN_PASSWORD must be set (see .env.example)")
	}
	secret := []byte(os.Getenv("SESSION_SECRET"))
	if len(secret) == 0 {
		// Without a configured secret, logins last until the next restart.
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := storage.Open(ctx, envOr("DB_PATH", "badminton.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Seed(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr: envOr("ADDR", ":8080"),
		Handler: httpapi.New(httpapi.Config{
			Sessions:      session.New(store, nil),
			Finance:       finance.New(store, nil),
			Web:           web.Dist(),
			AdminUsername: username,
			AdminPassword: password,
			Secret:        secret,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	log.Printf("listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads KEY=VALUE lines from path into the environment without
// overriding variables that are already set. A missing file is fine.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"'`)
		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
