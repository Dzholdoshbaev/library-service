package security

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"library-service/internal/domain"
)

func TestJWT(t *testing.T) {
	secret := strings.Repeat("k", 32)
	tokens := NewTokens(secret, "issuer", "audience", time.Minute)
	u := domain.User{ID: uuid.New(), Role: domain.Reader, AuthVersion: 1}
	raw, err := tokens.Issue(u)
	require.NoError(t, err)
	p, err := tokens.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, u.Principal(), p)
	for _, tc := range []struct {
		name   string
		tokens *Tokens
		raw    string
	}{{"wrong key", NewTokens(strings.Repeat("x", 32), "issuer", "audience", time.Minute), raw}, {"wrong issuer", NewTokens(secret, "other", "audience", time.Minute), raw}, {"wrong audience", NewTokens(secret, "issuer", "other", time.Minute), raw}, {"malformed", tokens, "bad.token.value"}} {
		t.Run(tc.name, func(t *testing.T) { _, err := tc.tokens.Parse(tc.raw); require.Error(t, err) })
	}
	expired, err := NewTokens(secret, "issuer", "audience", -time.Minute).Issue(u)
	require.NoError(t, err)
	_, err = tokens.Parse(expired)
	require.Error(t, err)
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodNone} {
		key := any([]byte(secret))
		if method == jwt.SigningMethodNone {
			key = jwt.UnsafeAllowNoneSignatureType
		}
		v, err := jwt.NewWithClaims(method, jwt.MapClaims{"sub": u.ID.String(), "iss": "issuer", "aud": "audience", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(key)
		require.NoError(t, err)
		_, err = tokens.Parse(v)
		require.Error(t, err)
	}
	a, hash, err := NewRefresh()
	require.NoError(t, err)
	b, _, err := NewRefresh()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.True(t, ValidRefresh(a))
	require.Len(t, hash, 64)
	require.Equal(t, hash, Fingerprint(a))
	require.False(t, ValidRefresh("bad"))
}
