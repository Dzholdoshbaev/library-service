package integration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"library-service/internal/app"
	"library-service/internal/domain"
	"library-service/internal/security"
)

func parallel(t *testing.T, n int, fn func(int) error) {
	t.Helper()
	start := make(chan struct{})
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); <-start; errs <- fn(i) }(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
func TestPostgresMigrationsAndConstraints(t *testing.T) {
	db, s, dsn := database(t)
	ctx := t.Context()
	admin := seedUser(t, s, domain.Admin)
	reader := seedUser(t, s, domain.Reader)
	b := seedBook(t, s, admin.Principal())
	got, err := s.UserByID(ctx, reader.ID)
	require.NoError(t, err)
	require.Equal(t, reader.ID, got.ID)
	duplicate := reader
	duplicate.ID = uuid.New()
	err = s.CreateUser(ctx, &duplicate)
	require.True(t, domain.HasCode(err, "USER_ALREADY_EXISTS"))
	require.Error(t, db.WithContext(ctx).Create(&domain.Favorite{ReaderID: uuid.New(), BookID: b.ID}).Error)
	require.Error(t, db.WithContext(ctx).Model(&domain.Book{}).Where("id = ?", b.ID).Update("likes_count", -1).Error)
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = s.BookByID(cancelCtx, b.ID)
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, s.SetLike(ctx, reader.Principal(), b.ID, true))
	require.NoError(t, s.SetFavorite(ctx, reader.Principal(), b.ID, true))
	c := domain.Comment{ID: uuid.New(), BookID: b.ID, ReaderID: reader.ID, Content: "Retained attribution"}
	require.NoError(t, s.CreateComment(ctx, reader.Principal(), &c))
	require.NoError(t, s.DeleteReader(ctx, admin.Principal(), reader.ID))
	_, err = s.UserByID(ctx, reader.ID)
	require.True(t, domain.HasCode(err, "USER_NOT_FOUND"))
	book, err := s.BookByID(ctx, b.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, book.LikesCount)
	require.EqualValues(t, 1, book.CommentsCount)
	require.NoError(t, s.DeleteBook(ctx, admin.Principal(), b.ID))
	for _, model := range []any{&domain.Comment{}, &domain.Like{}, &domain.Favorite{}} {
		var count int64
		require.NoError(t, db.WithContext(ctx).Model(model).Count(&count).Error)
		require.Zero(t, count)
	}
	require.NoError(t, app.Migrate(dsn, "down"))
	require.False(t, db.WithContext(ctx).Migrator().HasTable("users"))
	require.NoError(t, app.Migrate(dsn, "up"))
	require.True(t, db.WithContext(ctx).Migrator().HasTable("refresh_tokens"))
	require.NoError(t, app.Migrate(dsn, "up"))
}
func TestConcurrentCountersAndFavorites(t *testing.T) {
	db, s, _ := database(t)
	ctx := t.Context()
	admin := seedUser(t, s, domain.Admin)
	reader := seedUser(t, s, domain.Reader)
	b := seedBook(t, s, admin.Principal())
	parallel(t, 24, func(int) error { return s.SetLike(ctx, reader.Principal(), b.ID, true) })
	book, err := s.BookByID(ctx, b.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, book.LikesCount)
	parallel(t, 24, func(int) error { return s.SetFavorite(ctx, reader.Principal(), b.ID, true) })
	var rows int64
	require.NoError(t, db.WithContext(ctx).Model(&domain.Favorite{}).Count(&rows).Error)
	require.EqualValues(t, 1, rows)
	parallel(t, 24, func(int) error { return s.SetFavorite(ctx, reader.Principal(), b.ID, false) })
	parallel(t, 24, func(int) error { return s.SetLike(ctx, reader.Principal(), b.ID, false) })
	book, err = s.BookByID(ctx, b.ID)
	require.NoError(t, err)
	require.Zero(t, book.LikesCount)
	users := make([]domain.User, 20)
	comments := make([]domain.Comment, len(users))
	for i := range users {
		users[i] = seedUser(t, s, domain.Reader)
	}
	parallel(t, len(users), func(i int) error {
		if err := s.SetLike(ctx, users[i].Principal(), b.ID, true); err != nil {
			return err
		}
		comments[i] = domain.Comment{ID: uuid.New(), BookID: b.ID, Content: fmt.Sprintf("Comment %d", i)}
		return s.CreateComment(ctx, users[i].Principal(), &comments[i])
	})
	book, err = s.BookByID(ctx, b.ID)
	require.NoError(t, err)
	require.EqualValues(t, len(users), book.LikesCount)
	require.EqualValues(t, len(users), book.CommentsCount)
	listed, total, err := s.ListComments(ctx, b.ID, domain.Page{Number: 1, Size: 100, Sort: "created_at"})
	require.NoError(t, err)
	require.EqualValues(t, len(users), total)
	require.NotEmpty(t, listed[0].Reader.FirstName)
	require.Empty(t, listed[0].Reader.PasswordHash)
	parallel(t, len(users), func(i int) error {
		if err := s.SetLike(ctx, users[i].Principal(), b.ID, false); err != nil {
			return err
		}
		return s.DeleteComment(ctx, admin.Principal(), b.ID, comments[i].ID)
	})
	book, err = s.BookByID(ctx, b.ID)
	require.NoError(t, err)
	require.Zero(t, book.LikesCount)
	require.Zero(t, book.CommentsCount)
	err = s.DeleteComment(ctx, admin.Principal(), b.ID, comments[0].ID)
	require.True(t, domain.HasCode(err, "COMMENT_NOT_FOUND"))
	// A constraint failure rolls back both the inserted interaction and its counter.
	require.NoError(t, db.WithContext(ctx).Exec("ALTER TABLE books ADD CONSTRAINT reject_test_likes CHECK (likes_count = 0)").Error)
	require.Error(t, s.SetLike(ctx, reader.Principal(), b.ID, true))
	require.NoError(t, db.WithContext(ctx).Model(&domain.Like{}).Count(&rows).Error)
	require.Zero(t, rows)
}
func TestRefreshRotationReuseAndBlocking(t *testing.T) {
	db, s, _ := database(t)
	ctx := t.Context()
	admin := seedUser(t, s, domain.Admin)
	u := seedUser(t, s, domain.Reader)
	b := seedBook(t, s, admin.Principal())
	raw, hash, err := security.NewRefresh()
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	token := domain.RefreshToken{ID: uuid.New(), FamilyID: uuid.New(), TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}
	_, err = s.StartSession(ctx, u.ID, u.PasswordHash, &token)
	require.NoError(t, err)
	var next [2]domain.RefreshToken
	var outcomes [2]error
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range next {
		_, h, e := security.NewRefresh()
		require.NoError(t, e)
		next[i] = domain.RefreshToken{ID: uuid.New(), TokenHash: h}
		wg.Add(1)
		go func(i int) { defer wg.Done(); <-start; _, outcomes[i] = s.RotateSession(ctx, hash, &next[i]) }(i)
	}
	close(start)
	wg.Wait()
	winner := 0
	if outcomes[0] != nil {
		winner = 1
	}
	require.NoError(t, outcomes[winner])
	require.True(t, domain.HasCode(outcomes[1-winner], "UNAUTHORIZED"))
	successor := domain.RefreshToken{ID: uuid.New(), TokenHash: security.Fingerprint("not-used")}
	_, err = s.RotateSession(ctx, next[winner].TokenHash, &successor)
	require.True(t, domain.HasCode(err, "UNAUTHORIZED"))
	var active int64
	require.NoError(t, db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("revoked_at IS NULL").Count(&active).Error)
	require.Zero(t, active)
	require.NoError(t, s.SetReaderStatus(ctx, admin.Principal(), u.ID, domain.Blocked))
	require.True(t, domain.HasCode(s.SetLike(ctx, u.Principal(), b.ID, true), "USER_BLOCKED"))
	require.True(t, domain.HasCode(s.SetFavorite(ctx, u.Principal(), b.ID, true), "USER_BLOCKED"))
	_, err = s.StartSession(ctx, u.ID, u.PasswordHash, &token)
	require.True(t, domain.HasCode(err, "INVALID_CREDENTIALS"))
	require.NoError(t, s.SetReaderStatus(ctx, admin.Principal(), u.ID, domain.Active))
	require.True(t, domain.HasCode(s.SetLike(ctx, u.Principal(), b.ID, true), "UNAUTHORIZED"))
	current, err := s.UserByID(ctx, u.ID)
	require.NoError(t, err)
	require.NoError(t, s.SetLike(ctx, current.Principal(), b.ID, true))
}
