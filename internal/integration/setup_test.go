package integration_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"library-service/internal/app"
	"library-service/internal/config"
	"library-service/internal/domain"
	"library-service/internal/httpapi"
	"library-service/internal/security"
	"library-service/internal/service"
	"library-service/internal/store"
	"library-service/internal/telemetry"
)

func database(t *testing.T) (*gorm.DB, *store.Store, string) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostgreSQL integration tests")
	}
	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(u.Path, "_test"), "integration database name must end in _test")
	base, err := gorm.Open(postgres.Open(raw), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := base.DB()
	require.NoError(t, err)
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, base.WithContext(t.Context()).Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, base.WithContext(cleanup).Exec(`DROP SCHEMA "`+schema+`" CASCADE`).Error)
		require.NoError(t, pool.Close())
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	dsn := u.String()
	require.NoError(t, app.Migrate(dsn, "up"))
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NowFunc: func() time.Time { return time.Now().UTC() }})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(12)
	t.Cleanup(func() { require.NoError(t, sql.Close()) })
	return db, store.New(db), dsn
}
func seedUser(t *testing.T, s *store.Store, role domain.Role) domain.User {
	t.Helper()
	u := domain.User{ID: uuid.New(), FirstName: "Test", LastName: "Reader", BirthYear: 1990, Email: uuid.NewString() + "@example.com", PasswordHash: "integration-fixture", Role: role, Status: domain.Active, AuthVersion: 1}
	require.NoError(t, s.CreateUser(t.Context(), &u))
	return u
}
func seedBook(t *testing.T, s *store.Store, p domain.Principal) domain.Book {
	t.Helper()
	b := domain.Book{ID: uuid.New(), Title: "Concurrent Systems", Author: "Test Author", PublicationYear: 2020}
	require.NoError(t, s.CreateBook(t.Context(), p, &b))
	return b
}
func application(t *testing.T, db *gorm.DB, s *store.Store) (http.Handler, *service.Auth) {
	t.Helper()
	cfg := config.Config{Env: "test", AccessTTL: time.Minute, RefreshTTL: time.Hour, AuthConcurrency: 4}
	tokens := security.NewTokens(strings.Repeat("test-secret-", 4), "test", "api", cfg.AccessTTL)
	auth, err := service.NewAuth(s, s, security.Passwords{}, tokens, cfg.AccessTTL, cfg.RefreshTTL)
	require.NoError(t, err)
	tel, err := telemetry.New(t.Context(), "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	pool, err := db.DB()
	require.NoError(t, err)
	return httpapi.New(cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)), tel, pool, auth, service.NewUsers(s), service.NewBooks(s, s), service.NewComments(s)), auth
}
