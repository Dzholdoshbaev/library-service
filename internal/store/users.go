package store

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"library-service/internal/domain"
)

func (s *Store) CreateUser(ctx context.Context, u *domain.User) error {
	return translate(s.db.WithContext(ctx).Create(u).Error, "user")
}
func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := s.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&u).Error
	return u, translate(err, "user")
}
func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	var u domain.User
	err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&u).Error
	return u, translate(err, "user")
}
func (s *Store) ListReaders(ctx context.Context, f domain.UserFilter) ([]domain.User, int64, error) {
	q := s.db.WithContext(ctx).Model(&domain.User{}).Where("role = ? AND deleted_at IS NULL", domain.Reader)
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.FirstName != "" {
		q = q.Where("first_name ILIKE ?", contains(f.FirstName))
	}
	if f.LastName != "" {
		q = q.Where("last_name ILIKE ?", contains(f.LastName))
	}
	if f.Email != "" {
		q = q.Where("email ILIKE ?", contains(f.Email))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translate(err, "user")
	}
	list := []domain.User{}
	err := paginate(order(q, f.Page, "users", "created_at", "first_name", "last_name", "email", "birth_year"), f.Page).Find(&list).Error
	return list, total, translate(err, "user")
}
func (s *Store) UpdateProfile(ctx context.Context, p domain.Principal, id uuid.UUID, patch domain.ProfilePatch) (domain.User, error) {
	var result domain.User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Self updates take UPDATE directly, avoiding a SHARE-to-UPDATE lock upgrade.
		strength := "SHARE"
		if p.ID == id {
			strength = "UPDATE"
		}
		actor, err := lockUser(tx, p.ID, strength)
		if err != nil {
			return err
		}
		if err = checkActor(actor, p, domain.Admin, domain.Reader); err != nil {
			return err
		}
		result = actor
		if p.ID != id {
			if p.Role != domain.Admin {
				return domain.Forbidden()
			}
			result, err = lockUser(tx, id, "UPDATE")
			if err != nil {
				return err
			}
			if result.Role != domain.Reader {
				return domain.NotFound("user")
			}
		}
		updates := map[string]any{}
		if patch.FirstName != nil {
			updates["first_name"] = strings.TrimSpace(*patch.FirstName)
		}
		if patch.LastName != nil {
			updates["last_name"] = strings.TrimSpace(*patch.LastName)
		}
		if patch.MiddleName != nil {
			updates["middle_name"] = strings.TrimSpace(*patch.MiddleName)
		}
		if patch.Email != nil {
			updates["email"] = strings.ToLower(strings.TrimSpace(*patch.Email))
		}
		if patch.BirthYear != nil {
			updates["birth_year"] = *patch.BirthYear
		}
		if len(updates) > 0 {
			if err = tx.Model(&result).Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.First(&result, "id = ?", id).Error
	})
	return result, translate(err, "user")
}
func (s *Store) SetReaderStatus(ctx context.Context, p domain.Principal, id uuid.UUID, status domain.Status) error {
	return s.as(ctx, p, []domain.Role{domain.Admin}, func(tx *gorm.DB) error {
		u, err := lockUser(tx, id, "UPDATE")
		if err != nil {
			return err
		}
		if u.Role != domain.Reader {
			return domain.NotFound("user")
		}
		if u.Status == status {
			return nil
		}
		if err = tx.Model(&u).Updates(map[string]any{"status": status, "auth_version": gorm.Expr("auth_version + 1")}).Error; err != nil {
			return err
		}
		return revokeUser(tx, id)
	})
}
func (s *Store) DeleteReader(ctx context.Context, p domain.Principal, id uuid.UUID) error {
	return s.as(ctx, p, []domain.Role{domain.Admin}, func(tx *gorm.DB) error {
		u, err := lockUser(tx, id, "UPDATE")
		if err != nil {
			return err
		}
		if u.Role != domain.Reader {
			return domain.NotFound("user")
		}
		if err = tx.Model(&u).Updates(map[string]any{"deleted_at": time.Now().UTC(), "auth_version": gorm.Expr("auth_version + 1")}).Error; err != nil {
			return err
		}
		return revokeUser(tx, id)
	})
}
func revokeUser(tx *gorm.DB, id uuid.UUID) error {
	return tx.Model(&domain.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", id).Update("revoked_at", time.Now().UTC()).Error
}
