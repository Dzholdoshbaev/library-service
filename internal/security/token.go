package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"library-service/internal/domain"
)

type Tokens struct {
	secret   []byte
	ttl      time.Duration
	issuer   string
	audience string
}
type Claims struct {
	Role    domain.Role `json:"role"`
	Version int64       `json:"ver"`
	jwt.RegisteredClaims
}

func NewTokens(secret, issuer, audience string, ttl time.Duration) *Tokens {
	return &Tokens{[]byte(secret), ttl, issuer, audience}
}
func (t *Tokens) Issue(u domain.User) (string, error) {
	now := time.Now().UTC()
	c := Claims{u.Role, u.AuthVersion, jwt.RegisteredClaims{Subject: u.ID.String(), Issuer: t.issuer, Audience: jwt.ClaimStrings{t.audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl))}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return raw, nil
}
func (t *Tokens) Parse(raw string) (domain.Principal, error) {
	c := new(Claims)
	_, err := jwt.ParseWithClaims(raw, c, func(_ *jwt.Token) (any, error) { return t.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithIssuer(t.issuer), jwt.WithAudience(t.audience))
	if err != nil {
		return domain.Principal{}, domain.Unauthorized()
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil || id == uuid.Nil || c.Version < 1 || c.IssuedAt == nil || (c.Role != domain.Admin && c.Role != domain.Reader) {
		return domain.Principal{}, domain.Unauthorized()
	}
	return domain.Principal{ID: id, Role: c.Role, Version: c.Version}, nil
}
func NewRefresh() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw, Fingerprint(raw), nil
}
func Fingerprint(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func ValidRefresh(raw string) bool {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	return err == nil && len(b) == 32 && len(raw) == 43
}
