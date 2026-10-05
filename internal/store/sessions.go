package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"library-service/internal/domain"
)

func (s *Store) StartSession(ctx context.Context, id uuid.UUID, expectedHash string, row *domain.RefreshToken) (domain.User, error) {
	var u domain.User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		u, err = lockUser(tx, id, "SHARE")
		if err != nil {
			if domain.HasCode(err, "USER_NOT_FOUND") {
				return domain.Credentials()
			}
			return err
		}
		if u.Status != domain.Active || u.PasswordHash != expectedHash {
			return domain.Credentials()
		}
		row.UserID = id
		return tx.Create(row).Error
	})
	return u, translate(err, "user")
}
func (s *Store) RotateSession(ctx context.Context, hash string, next *domain.RefreshToken) (domain.User, error) {
	var u domain.User
	var rejected error
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lookup only to discover lock owner; re-read token after acquiring user lock.
		var current domain.RefreshToken
		if err := tx.Where("token_hash = ?", hash).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Unauthorized()
			}
			return err
		}
		var err error
		u, err = lockUser(tx, current.UserID, "UPDATE")
		if err != nil {
			if domain.HasCode(err, "USER_NOT_FOUND") {
				return domain.Unauthorized()
			}
			return err
		}
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", current.ID).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if u.Status != domain.Active || current.ConsumedAt != nil || current.RevokedAt != nil || !current.ExpiresAt.After(now) {
			rejected = domain.Unauthorized()
			// Return nil after revoking: returning the rejection here would roll back it.
			return tx.Model(&domain.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", current.FamilyID).Update("revoked_at", now).Error
		}
		if err = tx.Model(&current).Update("consumed_at", now).Error; err != nil {
			return err
		}
		next.UserID = u.ID
		next.FamilyID = current.FamilyID
		next.ExpiresAt = current.ExpiresAt
		return tx.Create(next).Error
	})
	if err != nil {
		return domain.User{}, translate(err, "user")
	}
	if rejected != nil {
		return domain.User{}, rejected
	}
	return u, nil
}
func (s *Store) RevokeSession(ctx context.Context, hash string) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token domain.RefreshToken
		if err := tx.Where("token_hash = ?", hash).First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		// Lock even tombstoned accounts so logout serializes with rotation.
		var u domain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", token.UserID).Error; err != nil {
			return err
		}
		return tx.Model(&domain.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", token.FamilyID).Update("revoked_at", time.Now().UTC()).Error
	}), "user")
}
func (s *Store) ChangePassword(ctx context.Context, p domain.Principal, oldHash, newHash string) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		u, err := lockUser(tx, p.ID, "UPDATE")
		if err != nil {
			return err
		}
		if err = checkActor(u, p, domain.Admin, domain.Reader); err != nil {
			return err
		}
		if u.PasswordHash != oldHash {
			return domain.Credentials()
		}
		if err = tx.Model(&u).Updates(map[string]any{"password_hash": newHash, "auth_version": gorm.Expr("auth_version + 1")}).Error; err != nil {
			return err
		}
		return revokeUser(tx, u.ID)
	}), "user")
}
