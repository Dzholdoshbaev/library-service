package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"library-service/internal/domain"
)

func (s *Store) CreateBook(ctx context.Context, p domain.Principal, b *domain.Book) error {
	return s.as(ctx, p, []domain.Role{domain.Admin}, func(tx *gorm.DB) error { return tx.Create(b).Error })
}
func (s *Store) BookByID(ctx context.Context, id uuid.UUID) (domain.Book, error) {
	var b domain.Book
	err := s.db.WithContext(ctx).First(&b, "id = ?", id).Error
	return b, translate(err, "book")
}
func (s *Store) ListBooks(ctx context.Context, f domain.BookFilter) ([]domain.Book, int64, error) {
	q := s.db.WithContext(ctx).Model(&domain.Book{})
	if f.Title != "" {
		q = q.Where("title ILIKE ?", contains(f.Title))
	}
	if f.Author != "" {
		q = q.Where("author ILIKE ?", contains(f.Author))
	}
	if f.Year != 0 {
		q = q.Where("publication_year = ?", f.Year)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translate(err, "book")
	}
	list := []domain.Book{}
	err := paginate(order(q, f.Page, "books", "created_at", "title", "author", "publication_year", "likes_count", "comments_count"), f.Page).Find(&list).Error
	return list, total, translate(err, "book")
}
func (s *Store) UpdateBook(ctx context.Context, p domain.Principal, id uuid.UUID, patch domain.BookPatch) (domain.Book, error) {
	var b domain.Book
	err := s.as(ctx, p, []domain.Role{domain.Admin}, func(tx *gorm.DB) error {
		var err error
		b, err = lockBook(tx, id)
		if err != nil {
			return err
		}
		updates := map[string]any{}
		if patch.Title != nil {
			updates["title"] = strings.TrimSpace(*patch.Title)
		}
		if patch.Author != nil {
			updates["author"] = strings.TrimSpace(*patch.Author)
		}
		if patch.PublicationYear != nil {
			updates["publication_year"] = *patch.PublicationYear
		}
		if patch.Volume != nil {
			updates["volume"] = *patch.Volume
		}
		if len(updates) > 0 {
			if err = tx.Model(&b).Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.First(&b, "id = ?", id).Error
	})
	return b, err
}
func (s *Store) DeleteBook(ctx context.Context, p domain.Principal, id uuid.UUID) error {
	return s.as(ctx, p, []domain.Role{domain.Admin}, func(tx *gorm.DB) error {
		b, err := lockBook(tx, id)
		if err != nil {
			return err
		}
		return tx.Delete(&b).Error
	})
}
