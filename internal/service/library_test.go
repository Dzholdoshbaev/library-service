package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"library-service/internal/domain"
)

type bookStub struct {
	BookRepository
	created domain.Book
	updates int
	deletes int
}

func (s *bookStub) CreateBook(_ context.Context, _ domain.Principal, b *domain.Book) error {
	s.created = *b
	return nil
}
func (s *bookStub) UpdateBook(_ context.Context, _ domain.Principal, _ uuid.UUID, _ domain.BookPatch) (domain.Book, error) {
	s.updates++
	return s.created, nil
}
func (s *bookStub) DeleteBook(context.Context, domain.Principal, uuid.UUID) error {
	s.deletes++
	return nil
}

type interactionStub struct {
	InteractionRepository
	favoriteCalls, likeCalls int
	on                       bool
}

func (s *interactionStub) SetFavorite(_ context.Context, _ domain.Principal, _ uuid.UUID, on bool) error {
	s.favoriteCalls++
	s.on = on
	return nil
}
func (s *interactionStub) SetLike(_ context.Context, _ domain.Principal, _ uuid.UUID, on bool) error {
	s.likeCalls++
	s.on = on
	return nil
}
func TestBookServices(t *testing.T) {
	repo := &bookStub{}
	interactions := &interactionStub{}
	svc := NewBooks(repo, interactions)
	admin := domain.Principal{ID: uuid.New(), Role: domain.Admin, Version: 1}
	reader := domain.Principal{ID: uuid.New(), Role: domain.Reader, Version: 1}
	b := domain.Book{Title: " Clean Code ", Author: "Robert Martin", PublicationYear: 2008, LikesCount: 99, CommentsCount: 77}
	_, err := svc.Create(t.Context(), reader, b)
	require.True(t, domain.HasCode(err, "FORBIDDEN"))
	require.Equal(t, uuid.Nil, repo.created.ID)
	out, err := svc.Create(t.Context(), admin, b)
	require.NoError(t, err)
	require.Equal(t, "Clean Code", out.Title)
	require.Zero(t, out.LikesCount)
	require.Zero(t, out.CommentsCount)
	blank := " "
	_, err = svc.Update(t.Context(), admin, out.ID, domain.BookPatch{Title: &blank})
	require.Error(t, err)
	require.Zero(t, repo.updates)
	title := "Revised"
	_, err = svc.Update(t.Context(), admin, out.ID, domain.BookPatch{Title: &title})
	require.NoError(t, err)
	require.Equal(t, 1, repo.updates)
	require.Error(t, svc.Delete(t.Context(), reader, out.ID))
	require.Zero(t, repo.deletes)
	require.NoError(t, svc.Delete(t.Context(), admin, out.ID))
	for _, on := range []bool{true, false} {
		require.NoError(t, svc.Favorite(t.Context(), reader, out.ID, on))
		require.Equal(t, on, interactions.on)
		require.NoError(t, svc.Like(t.Context(), reader, out.ID, on))
		require.Equal(t, on, interactions.on)
	}
	require.Error(t, svc.Favorite(t.Context(), admin, out.ID, true))
	require.Error(t, svc.Like(t.Context(), admin, out.ID, true))
	require.Equal(t, 2, interactions.favoriteCalls)
	require.Equal(t, 2, interactions.likeCalls)
}

type commentStub struct {
	CommentRepository
	comment                   domain.Comment
	creates, updates, deletes int
}

func (s *commentStub) CommentByID(context.Context, uuid.UUID, uuid.UUID) (domain.Comment, error) {
	return s.comment, nil
}
func (s *commentStub) CreateComment(_ context.Context, _ domain.Principal, c *domain.Comment) error {
	s.creates++
	s.comment = *c
	return nil
}
func (s *commentStub) UpdateComment(_ context.Context, _ domain.Principal, _, _ uuid.UUID, content string) (domain.Comment, error) {
	s.updates++
	s.comment.Content = content
	return s.comment, nil
}
func (s *commentStub) DeleteComment(context.Context, domain.Principal, uuid.UUID, uuid.UUID) error {
	s.deletes++
	return nil
}
func TestCommentOwnership(t *testing.T) {
	owner := domain.Principal{ID: uuid.New(), Role: domain.Reader, Version: 1}
	other := domain.Principal{ID: uuid.New(), Role: domain.Reader, Version: 1}
	admin := domain.Principal{ID: uuid.New(), Role: domain.Admin, Version: 1}
	book := uuid.New()
	repo := &commentStub{}
	svc := NewComments(repo)
	c, err := svc.Create(t.Context(), owner, book, " First comment ")
	require.NoError(t, err)
	require.Equal(t, owner.ID, c.ReaderID)
	require.Equal(t, "First comment", c.Content)
	_, err = svc.Create(t.Context(), admin, book, "forbidden")
	require.Error(t, err)
	_, err = svc.Create(t.Context(), owner, book, " ")
	require.Error(t, err)
	require.Equal(t, 1, repo.creates)
	_, err = svc.Update(t.Context(), other, book, c.ID, "replacement")
	require.True(t, domain.HasCode(err, "FORBIDDEN"))
	require.Zero(t, repo.updates)
	_, err = svc.Update(t.Context(), owner, book, c.ID, " replacement ")
	require.NoError(t, err)
	require.Equal(t, "replacement", repo.comment.Content)
	require.Error(t, svc.Delete(t.Context(), other, book, c.ID))
	require.Zero(t, repo.deletes)
	require.NoError(t, svc.Delete(t.Context(), owner, book, c.ID))
	require.NoError(t, svc.Delete(t.Context(), admin, book, c.ID))
	require.Equal(t, 2, repo.deletes)
}
