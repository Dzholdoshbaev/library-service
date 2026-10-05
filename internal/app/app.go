package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"library-service/internal/config"
	"library-service/internal/httpapi"
	"library-service/internal/security"
	"library-service/internal/service"
	"library-service/internal/store"
	"library-service/internal/telemetry"
)

func Run(ctx context.Context, args []string, out io.Writer) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}
	if command == "healthcheck" {
		return healthcheck(ctx)
	}
	if command != "serve" && command != "migrate" && command != "bootstrap-admin" {
		return errors.New("usage: library [serve | migrate up|down | bootstrap-admin | healthcheck]")
	}
	if command == "migrate" && (len(args) != 2 || (args[1] != "up" && args[1] != "down")) {
		return errors.New("usage: library migrate up|down")
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	options := &slog.HandlerOptions{Level: c.LogLevel}
	var handler slog.Handler = slog.NewJSONHandler(out, options)
	if c.Env == "development" {
		handler = slog.NewTextHandler(out, options)
	}
	log := slog.New(handler)
	t, err := telemetry.New(ctx, c.OTLPEndpoint)
	if err != nil {
		return errors.New("initialize telemetry failed")
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := t.Shutdown(shutdown); err != nil {
			log.Error("telemetry shutdown failed")
		}
	}()
	db, pool, err := store.Open(ctx, c, telemetry.SQLLogger{Telemetry: t})
	if err != nil {
		return errors.New("PostgreSQL connection failed; check configuration and availability")
	}
	defer func() {
		if err := pool.Close(); err != nil {
			log.Error("database pool close failed")
		}
	}()
	t.Pool(pool)
	if command == "migrate" {
		if err := Migrate(c.DatabaseURL, args[1]); err != nil {
			return errors.New("migration failed; inspect migration version and database availability")
		}
		log.Info("migrations applied", "direction", args[1])
		return nil
	}
	repo := store.New(db)
	tokens := security.NewTokens(c.AccessSecret, c.Issuer, c.Audience, c.AccessTTL)
	auth, err := service.NewAuth(repo, repo, security.Passwords{}, tokens, c.AccessTTL, c.RefreshTTL)
	if err != nil {
		return err
	}
	if command == "bootstrap-admin" {
		year, err := strconv.Atoi(os.Getenv("ADMIN_BIRTH_YEAR"))
		if err != nil {
			return errors.New("ADMIN_BIRTH_YEAR must be provided as a number")
		}
		user, err := auth.Bootstrap(ctx, service.Registration{FirstName: os.Getenv("ADMIN_FIRST_NAME"), LastName: os.Getenv("ADMIN_LAST_NAME"), Email: os.Getenv("ADMIN_EMAIL"), Password: os.Getenv("ADMIN_PASSWORD"), BirthYear: year})
		if err != nil {
			return fmt.Errorf("bootstrap admin: %w", err)
		}
		log.Info("admin created", "user_id", user.ID)
		return nil
	}
	router := httpapi.New(c, log, t, pool, auth, service.NewUsers(repo), service.NewBooks(repo, repo), service.NewComments(repo))
	server := &http.Server{Addr: c.Address, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	stopped := make(chan error, 1)
	go func() { stopped <- server.ListenAndServe() }()
	log.Info("HTTP server started", "address", c.Address)
	select {
	case err := <-stopped:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
	}
	log.Info("shutting down HTTP server")
	// A new bounded context is required because the process signal canceled ctx.
	shutdown, cancel := context.WithTimeout(context.Background(), c.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return fmt.Errorf("HTTP shutdown deadline: %w", err)
	}
	return nil
}
func healthcheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/health/ready", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New("health check failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("service not ready")
	}
	return nil
}
