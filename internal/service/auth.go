package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"library-service/internal/domain"
	"library-service/internal/security"
)

type Accounts interface {
	CreateUser(context.Context, *domain.User) error
	UserByEmail(context.Context, string) (domain.User, error)
	UserByID(context.Context, uuid.UUID) (domain.User, error)
}
type Sessions interface {
	StartSession(context.Context, uuid.UUID, string, *domain.RefreshToken) (domain.User, error)
	RotateSession(context.Context, string, *domain.RefreshToken) (domain.User, error)
	RevokeSession(context.Context, string) error
	ChangePassword(context.Context, domain.Principal, string, string) error
}
type PasswordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) bool
}
type Auth struct {
	users      Accounts
	sessions   Sessions
	passwords  PasswordHasher
	tokens     *security.Tokens
	refreshTTL time.Duration
	accessTTL  time.Duration
	dummyHash  string
}
type Credentials struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	User         domain.User
}
type Registration struct {
	FirstName, LastName string
	MiddleName          *string
	BirthYear           int
	Email, Password     string
}

func NewAuth(users Accounts, sessions Sessions, passwords PasswordHasher, tokens *security.Tokens, accessTTL, refreshTTL time.Duration) (*Auth, error) {
	dummy, err := passwords.Hash("timing-equalizer-never-an-account")
	if err != nil {
		return nil, fmt.Errorf("initialize password verifier: %w", err)
	}
	return &Auth{users, sessions, passwords, tokens, refreshTTL, accessTTL, dummy}, nil
}
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func (a *Auth) Register(ctx context.Context, in Registration) (domain.User, error) {
	return a.create(ctx, in, domain.Reader)
}
func (a *Auth) Bootstrap(ctx context.Context, in Registration) (domain.User, error) {
	return a.create(ctx, in, domain.Admin)
}
func (a *Auth) create(ctx context.Context, in Registration, role domain.Role) (domain.User, error) {
	if err := validateRegistration(in); err != nil {
		return domain.User{}, err
	}
	hash, err := a.passwords.Hash(in.Password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}
	u := domain.User{ID: uuid.New(), FirstName: strings.TrimSpace(in.FirstName), LastName: strings.TrimSpace(in.LastName), MiddleName: trimOptional(in.MiddleName), BirthYear: in.BirthYear, Email: NormalizeEmail(in.Email), PasswordHash: hash, Role: role, Status: domain.Active, AuthVersion: 1}
	if err := a.users.CreateUser(ctx, &u); err != nil {
		return domain.User{}, err
	}
	return u, nil
}
func (a *Auth) Login(ctx context.Context, email, password string) (Credentials, error) {
	u, err := a.users.UserByEmail(ctx, NormalizeEmail(email))
	if err != nil {
		if domain.HasCode(err, "USER_NOT_FOUND") {
			a.passwords.Verify(a.dummyHash, password)
			return Credentials{}, domain.Credentials()
		}
		return Credentials{}, err
	}
	if !a.passwords.Verify(u.PasswordHash, password) || u.Status != domain.Active {
		return Credentials{}, domain.Credentials()
	}
	raw, row, err := a.newSession()
	if err != nil {
		return Credentials{}, err
	}
	row.UserID = u.ID
	current, err := a.sessions.StartSession(ctx, u.ID, u.PasswordHash, &row)
	if err != nil {
		return Credentials{}, err
	}
	return a.credentials(current, raw)
}
func (a *Auth) newSession() (string, domain.RefreshToken, error) {
	raw, hash, err := security.NewRefresh()
	if err != nil {
		return "", domain.RefreshToken{}, err
	}
	return raw, domain.RefreshToken{ID: uuid.New(), FamilyID: uuid.New(), TokenHash: hash, ExpiresAt: time.Now().UTC().Add(a.refreshTTL)}, nil
}
func (a *Auth) credentials(u domain.User, refresh string) (Credentials, error) {
	access, err := a.tokens.Issue(u)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{access, refresh, int64(a.accessTTL.Seconds()), u}, nil
}
func (a *Auth) Refresh(ctx context.Context, raw string) (Credentials, error) {
	if !security.ValidRefresh(raw) {
		return Credentials{}, domain.Unauthorized()
	}
	next, row, err := a.newSession()
	if err != nil {
		return Credentials{}, err
	}
	u, err := a.sessions.RotateSession(ctx, security.Fingerprint(raw), &row)
	if err != nil {
		return Credentials{}, err
	}
	return a.credentials(u, next)
}
func (a *Auth) Logout(ctx context.Context, raw string) error {
	if !security.ValidRefresh(raw) {
		return domain.Unauthorized()
	}
	return a.sessions.RevokeSession(ctx, security.Fingerprint(raw))
}
func (a *Auth) Authenticate(ctx context.Context, raw string) (domain.Principal, error) {
	p, err := a.tokens.Parse(raw)
	if err != nil {
		return domain.Principal{}, err
	}
	u, err := a.users.UserByID(ctx, p.ID)
	if err != nil {
		if domain.HasCode(err, "USER_NOT_FOUND") {
			return domain.Principal{}, domain.Unauthorized()
		}
		return domain.Principal{}, err
	}
	if u.Status == domain.Blocked {
		return domain.Principal{}, domain.UserBlocked()
	}
	if u.AuthVersion != p.Version || u.Role != p.Role {
		return domain.Principal{}, domain.Unauthorized()
	}
	return u.Principal(), nil
}
func (a *Auth) Password(ctx context.Context, p domain.Principal, current, next string) error {
	if !security.ValidatePassword(next) {
		return domain.Validation(map[string]string{"new_password": "must contain at least 12 characters and at most 128 bytes"})
	}
	u, err := a.users.UserByID(ctx, p.ID)
	if err != nil {
		return err
	}
	if !a.passwords.Verify(u.PasswordHash, current) {
		return domain.Credentials()
	}
	hash, err := a.passwords.Hash(next)
	if err != nil {
		return fmt.Errorf("hash replacement password: %w", err)
	}
	return a.sessions.ChangePassword(ctx, p, u.PasswordHash, hash)
}
