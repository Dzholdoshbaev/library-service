package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"library-service/internal/domain"
)

type CommentRepository interface {
	CreateComment(context.Context, domain.Principal, *domain.Comment) error
	CommentByID(context.Context, uuid.UUID, uuid.UUID) (domain.Comment, error)
	ListComments(context.Context, uuid.UUID, domain.Page) ([]domain.Comment, int64, error)
	UpdateComment(context.Context, domain.Principal, uuid.UUID, uuid.UUID, string) (domain.Comment, error)
	DeleteComment(context.Context, domain.Principal, uuid.UUID, uuid.UUID) error
}
type Comments struct{ repo CommentRepository }

func NewComments(repo CommentRepository) *Comments { return &Comments{repo} }
func (s *Comments) Create(ctx context.Context, p domain.Principal, book uuid.UUID, content string) (domain.Comment, error) {
	if err := domain.Allow(p, domain.Reader); err != nil {
		return domain.Comment{}, err
	}
	if !bounded(content, 4000) {
		return domain.Comment{}, domain.Validation(map[string]string{"content": "must contain 1 to 4000 characters"})
	}
	c := domain.Comment{ID: uuid.New(), BookID: book, ReaderID: p.ID, Content: strings.TrimSpace(content)}
	err := s.repo.CreateComment(ctx, p, &c)
	return c, err
}
func (s *Comments) List(ctx context.Context, id uuid.UUID, page domain.Page) ([]domain.Comment, int64, error) {
	return s.repo.ListComments(ctx, id, page)
}
func (s *Comments) Update(ctx context.Context, p domain.Principal, book, id uuid.UUID, content string) (domain.Comment, error) {
	if err := domain.Allow(p, domain.Reader); err != nil {
		return domain.Comment{}, err
	}
	if !bounded(content, 4000) {
		return domain.Comment{}, domain.Validation(map[string]string{"content": "must contain 1 to 4000 characters"})
	}
	c, err := s.repo.CommentByID(ctx, book, id)
	if err != nil {
		return c, err
	}
	if c.ReaderID != p.ID {
		return domain.Comment{}, domain.Forbidden()
	}
	return s.repo.UpdateComment(ctx, p, book, id, strings.TrimSpace(content))
}
func (s *Comments) Delete(ctx context.Context, p domain.Principal, book, id uuid.UUID) error {
	if err := domain.Allow(p, domain.Admin, domain.Reader); err != nil {
		return err
	}
	c, err := s.repo.CommentByID(ctx, book, id)
	if err != nil {
		return err
	}
	if p.Role != domain.Admin && c.ReaderID != p.ID {
		return domain.Forbidden()
	}
	return s.repo.DeleteComment(ctx, p, book, id)
}
