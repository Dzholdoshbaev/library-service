package store

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"library-service/internal/domain"
)

func (s *Store) SetFavorite(ctx context.Context, p domain.Principal, id uuid.UUID, on bool) error {
	return s.as(ctx, p, []domain.Role{domain.Reader}, func(tx *gorm.DB) error {
		if _, err := lockBook(tx, id); err != nil {
			return err
		}
		if on {
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.Favorite{ReaderID: p.ID, BookID: id}).Error
		}
		return tx.Where("reader_id = ? AND book_id = ?", p.ID, id).Delete(&domain.Favorite{}).Error
	})
}
func (s *Store) SetLike(ctx context.Context, p domain.Principal, id uuid.UUID, on bool) error {
	return s.as(ctx, p, []domain.Role{domain.Reader}, func(tx *gorm.DB) error {
		if _, err := lockBook(tx, id); err != nil {
			return err
		}
		var result *gorm.DB
		delta := 1
		if on {
			result = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.Like{ReaderID: p.ID, BookID: id})
		} else {
			delta = -1
			result = tx.Where("reader_id = ? AND book_id = ?", p.ID, id).Delete(&domain.Like{})
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&domain.Book{}).Where("id = ?", id).Update("likes_count", gorm.Expr("likes_count + ?", delta)).Error
	})
}
func (s *Store) ListFavorites(ctx context.Context, p domain.Principal, page domain.Page) ([]domain.Book, int64, error) {
	q := s.db.WithContext(ctx).Model(&domain.Book{}).Joins("JOIN reader_favorites AS f ON f.book_id = books.id").Where("f.reader_id = ?", p.ID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translate(err, "book")
	}
	list := []domain.Book{}
	err := paginate(order(q.Select("books.*"), page, "books", "created_at", "title", "author", "publication_year", "likes_count", "comments_count"), page).Find(&list).Error
	return list, total, translate(err, "book")
}
