package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"library-service/internal/domain"
	"library-service/internal/security"
)

type testHasher struct{}

func (testHasher) Hash(p string) (string, error) { return "unit-hash:" + p, nil }
func (testHasher) Verify(h, p string) bool       { return h == "unit-hash:"+p }

type authMemory struct {
	u       domain.User
	exists  bool
	session domain.RefreshToken
	revoked bool
	changed bool
}

func (m *authMemory) CreateUser(_ context.Context, u *domain.User) error {
	if m.exists {
		return domain.E("USER_ALREADY_EXISTS", "email is already registered", 409)
	}
	m.u = *u
	m.exists = true
	return nil
}
func (m *authMemory) UserByEmail(context.Context, string) (domain.User, error) {
	if !m.exists {
		return domain.User{}, domain.NotFound("user")
	}
	return m.u, nil
}
func (m *authMemory) UserByID(ctx context.Context, _ uuid.UUID) (domain.User, error) {
	return m.UserByEmail(ctx, "")
}
func (m *authMemory) StartSession(_ context.Context, _ uuid.UUID, _ string, row *domain.RefreshToken) (domain.User, error) {
	m.session = *row
	return m.u, nil
}
func (m *authMemory) RotateSession(_ context.Context, hash string, row *domain.RefreshToken) (domain.User, error) {
	if m.revoked || hash != m.session.TokenHash {
		return domain.User{}, domain.Unauthorized()
	}
	m.session = *row
	return m.u, nil
}
func (m *authMemory) RevokeSession(context.Context, string) error { m.revoked = true; return nil }
func (m *authMemory) ChangePassword(_ context.Context, _ domain.Principal, _, hash string) error {
	m.u.PasswordHash = hash
	m.u.AuthVersion++
	m.changed = true
	return nil
}
func TestAuthLifecycle(t *testing.T) {
	ctx := t.Context()
	m := &authMemory{}
	tokens := security.NewTokens(strings.Repeat("s", 32), "test", "api", time.Minute)
	auth, err := NewAuth(m, m, testHasher{}, tokens, time.Minute, time.Hour)
	require.NoError(t, err)
	input := Registration{FirstName: " Ada ", LastName: "Lovelace", BirthYear: 1990, Email: " ADA@EXAMPLE.COM ", Password: "long-test-password"}
	u, err := auth.Register(ctx, input)
	require.NoError(t, err)
	require.Equal(t, domain.Reader, u.Role)
	require.Equal(t, "ada@example.com", u.Email)
	require.Equal(t, "Ada", u.FirstName)
	require.NotEqual(t, input.Password, u.PasswordHash)
	_, err = auth.Register(ctx, input)
	require.True(t, domain.HasCode(err, "USER_ALREADY_EXISTS"))
	creds, err := auth.Login(ctx, input.Email, input.Password)
	require.NoError(t, err)
	require.NotEqual(t, creds.RefreshToken, m.session.TokenHash)
	require.Equal(t, security.Fingerprint(creds.RefreshToken), m.session.TokenHash)
	p, err := auth.Authenticate(ctx, creds.AccessToken)
	require.NoError(t, err)
	require.Equal(t, u.ID, p.ID)
	_, err = auth.Login(ctx, input.Email, "wrong")
	require.True(t, domain.HasCode(err, "INVALID_CREDENTIALS"))
	m.u.Status = domain.Blocked
	_, err = auth.Login(ctx, input.Email, input.Password)
	require.True(t, domain.HasCode(err, "INVALID_CREDENTIALS"))
	_, err = auth.Authenticate(ctx, creds.AccessToken)
	require.True(t, domain.HasCode(err, "USER_BLOCKED"))
	m.u.Status = domain.Active
	next, err := auth.Refresh(ctx, creds.RefreshToken)
	require.NoError(t, err)
	require.NotEqual(t, creds.RefreshToken, next.RefreshToken)
	_, err = auth.Refresh(ctx, creds.RefreshToken)
	require.Error(t, err)
	require.NoError(t, auth.Logout(ctx, next.RefreshToken))
	_, err = auth.Refresh(ctx, next.RefreshToken)
	require.Error(t, err)
	require.Error(t, auth.Password(ctx, p, "wrong", "replacement-secret"))
	require.False(t, m.changed)
	require.NoError(t, auth.Password(ctx, p, input.Password, "replacement-secret"))
	require.True(t, m.changed)
	_, err = auth.Authenticate(ctx, creds.AccessToken)
	require.True(t, domain.HasCode(err, "UNAUTHORIZED"))
}
func TestRegistrationValidationAndUnknownLogin(t *testing.T) {
	m := &authMemory{}
	a, err := NewAuth(m, m, testHasher{}, security.NewTokens(strings.Repeat("s", 32), "test", "api", time.Minute), time.Minute, time.Hour)
	require.NoError(t, err)
	_, err = a.Register(t.Context(), Registration{})
	require.True(t, domain.HasCode(err, "VALIDATION_ERROR"))
	require.False(t, m.exists)
	_, err = a.Login(t.Context(), "missing@example.com", "anything")
	require.True(t, domain.HasCode(err, "INVALID_CREDENTIALS"))
}
