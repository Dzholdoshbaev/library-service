package integration_test

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"library-service/internal/domain"
	"library-service/internal/security"
	"testing"
	"time"
)

func TestExpiredRefreshAndPasswordInvalidation(t *testing.T) {
	db, s, _ := database(t)
	ctx := t.Context()
	u := seedUser(t, s, domain.Reader)
	makeToken := func() domain.RefreshToken {
		_, hash, err := security.NewRefresh()
		require.NoError(t, err)
		return domain.RefreshToken{ID: uuid.New(), FamilyID: uuid.New(), TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}
	}
	expired := makeToken()
	expired.CreatedAt = time.Now().Add(-2 * time.Hour)
	expired.ExpiresAt = time.Now().Add(-time.Hour)
	_, err := s.StartSession(ctx, u.ID, u.PasswordHash, &expired)
	require.NoError(t, err)
	next := makeToken()
	_, err = s.RotateSession(ctx, expired.TokenHash, &next)
	require.True(t, domain.HasCode(err, "UNAUTHORIZED"))
	active := makeToken()
	_, err = s.StartSession(ctx, u.ID, u.PasswordHash, &active)
	require.NoError(t, err)
	require.NoError(t, s.ChangePassword(ctx, u.Principal(), u.PasswordHash, "new-fixture-hash"))
	stale := makeToken()
	_, err = s.StartSession(ctx, u.ID, u.PasswordHash, &stale)
	require.True(t, domain.HasCode(err, "INVALID_CREDENTIALS"))
	next = makeToken()
	_, err = s.RotateSession(ctx, active.TokenHash, &next)
	require.True(t, domain.HasCode(err, "UNAUTHORIZED"))
	var live int64
	require.NoError(t, db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("revoked_at IS NULL").Count(&live).Error)
	require.Zero(t, live)
}
