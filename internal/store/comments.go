package store

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"library-service/internal/domain"
)

func commentQuery(q *gorm.DB) *gorm.DB {
	return q.Preload("Reader", func(tx *gorm.DB) *gorm.DB { return tx.Select("id", "first_name", "last_name", "deleted_at") })
}
func (s *Store) CommentByID(ctx context.Context, book, id uuid.UUID) (domain.Comment, error) {
	var c domain.Comment
	err := commentQuery(s.db.WithContext(ctx)).Where("id = ? AND book_id = ?", id, book).First(&c).Error
	return c, translate(err, "comment")
}
func (s *Store) CreateComment(ctx context.Context, p domain.Principal, c *domain.Comment) error {
	return s.as(ctx, p, []domain.Role{domain.Reader}, func(tx *gorm.DB) error {
		if _, err := lockBook(tx, c.BookID); err != nil {
			return err
		}
		c.ReaderID = p.ID
		if err := tx.Omit("Reader").Create(c).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Book{}).Where("id = ?", c.BookID).Update("comments_count", gorm.Expr("comments_count + 1")).Error; err != nil {
			return err
		}
		return commentQuery(tx).First(c, "id = ?", c.ID).Error
	})
}
func (s *Store) ListComments(ctx context.Context, book uuid.UUID, page domain.Page) ([]domain.Comment, int64, error) {
	if _, err := s.BookByID(ctx, book); err != nil {
		return nil, 0, err
	}
	q := s.db.WithContext(ctx).Model(&domain.Comment{}).Where("book_id = ?", book)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translate(err, "comment")
	}
	list := []domain.Comment{}
	err := paginate(order(commentQuery(q), page, "comments", "created_at", "updated_at"), page).Find(&list).Error
	return list, total, translate(err, "comment")
}
func (s *Store) UpdateComment(ctx context.Context, p domain.Principal, book, id uuid.UUID, content string) (domain.Comment, error) {
	var c domain.Comment
	err := s.as(ctx, p, []domain.Role{domain.Reader}, func(tx *gorm.DB) error {
		if _, err := lockBook(tx, book); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND book_id = ?", id, book).First(&c).Error; err != nil {
			return translate(err, "comment")
		}
		if c.ReaderID != p.ID {
			return domain.Forbidden()
		}
		if err := tx.Model(&c).Update("content", content).Error; err != nil {
			return err
		}
		return commentQuery(tx).First(&c, "id = ?", id).Error
	})
	return c, err
}
func (s *Store) DeleteComment(ctx context.Context, p domain.Principal, book, id uuid.UUID) error {
	return s.as(ctx, p, []domain.Role{domain.Reader, domain.Admin}, func(tx *gorm.DB) error {
		if _, err := lockBook(tx, book); err != nil {
			return err
		}
		var c domain.Comment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND book_id = ?", id, book).First(&c).Error; err != nil {
			return translate(err, "comment")
		}
		if p.Role != domain.Admin && c.ReaderID != p.ID {
			return domain.Forbidden()
		}
		if err := tx.Delete(&c).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Book{}).Where("id = ?", book).Update("comments_count", gorm.Expr("comments_count - 1")).Error
	})
}
