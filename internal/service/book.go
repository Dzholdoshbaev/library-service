package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"library-service/internal/domain"
)

type BookRepository interface {
	CreateBook(context.Context, domain.Principal, *domain.Book) error
	UpdateBook(context.Context, domain.Principal, uuid.UUID, domain.BookPatch) (domain.Book, error)
	DeleteBook(context.Context, domain.Principal, uuid.UUID) error
	BookByID(context.Context, uuid.UUID) (domain.Book, error)
	ListBooks(context.Context, domain.BookFilter) ([]domain.Book, int64, error)
}
type InteractionRepository interface {
	SetFavorite(context.Context, domain.Principal, uuid.UUID, bool) error
	SetLike(context.Context, domain.Principal, uuid.UUID, bool) error
	ListFavorites(context.Context, domain.Principal, domain.Page) ([]domain.Book, int64, error)
}
type Books struct {
	repo         BookRepository
	interactions InteractionRepository
}

func NewBooks(repo BookRepository, interactions InteractionRepository) *Books {
	return &Books{repo, interactions}
}
func (s *Books) Create(ctx context.Context, p domain.Principal, b domain.Book) (domain.Book, error) {
	if err := domain.Allow(p, domain.Admin); err != nil {
		return domain.Book{}, err
	}
	if err := validateBook(domain.BookPatch{Title: &b.Title, Author: &b.Author, PublicationYear: &b.PublicationYear, Volume: b.Volume}); err != nil {
		return domain.Book{}, err
	}
	book := domain.Book{ID: uuid.New(), Title: strings.TrimSpace(b.Title), Author: strings.TrimSpace(b.Author), PublicationYear: b.PublicationYear, Volume: b.Volume}
	err := s.repo.CreateBook(ctx, p, &book)
	return book, err
}
func (s *Books) Update(ctx context.Context, p domain.Principal, id uuid.UUID, patch domain.BookPatch) (domain.Book, error) {
	if err := domain.Allow(p, domain.Admin); err != nil {
		return domain.Book{}, err
	}
	if err := validateBook(patch); err != nil {
		return domain.Book{}, err
	}
	return s.repo.UpdateBook(ctx, p, id, patch)
}
func (s *Books) Delete(ctx context.Context, p domain.Principal, id uuid.UUID) error {
	if err := domain.Allow(p, domain.Admin); err != nil {
		return err
	}
	return s.repo.DeleteBook(ctx, p, id)
}
func (s *Books) Get(ctx context.Context, id uuid.UUID) (domain.Book, error) {
	return s.repo.BookByID(ctx, id)
}
func (s *Books) List(ctx context.Context, f domain.BookFilter) ([]domain.Book, int64, error) {
	return s.repo.ListBooks(ctx, f)
}
func (s *Books) Favorite(ctx context.Context, p domain.Principal, id uuid.UUID, on bool) error {
	if err := domain.Allow(p, domain.Reader); err != nil {
		return err
	}
	return s.interactions.SetFavorite(ctx, p, id, on)
}
func (s *Books) Like(ctx context.Context, p domain.Principal, id uuid.UUID, on bool) error {
	if err := domain.Allow(p, domain.Reader); err != nil {
		return err
	}
	return s.interactions.SetLike(ctx, p, id, on)
}
func (s *Books) Favorites(ctx context.Context, p domain.Principal, page domain.Page) ([]domain.Book, int64, error) {
	if err := domain.Allow(p, domain.Reader); err != nil {
		return nil, 0, err
	}
	return s.interactions.ListFavorites(ctx, p, page)
}
