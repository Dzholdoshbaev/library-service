package store

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"library-service/internal/domain"
)

func lockUser(tx *gorm.DB, id uuid.UUID, strength string) (domain.User, error) {
	var u domain.User
	err := tx.Clauses(clause.Locking{Strength: strength}).Where("id = ? AND deleted_at IS NULL", id).First(&u).Error
	return u, translate(err, "user")
}
func checkActor(u domain.User, p domain.Principal, roles ...domain.Role) error {
	if u.Status != domain.Active {
		return domain.UserBlocked()
	}
	if u.AuthVersion != p.Version || u.Role != p.Role {
		return domain.Unauthorized()
	}
	return domain.Allow(u.Principal(), roles...)
}
func (s *Store) as(ctx context.Context, p domain.Principal, roles []domain.Role, fn func(*gorm.DB) error) error {
	return translate(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		u, err := lockUser(tx, p.ID, "SHARE")
		if err != nil {
			return err
		}
		if err = checkActor(u, p, roles...); err != nil {
			return err
		}
		return fn(tx)
	}), "user")
}
func lockBook(tx *gorm.DB, id uuid.UUID) (domain.Book, error) {
	var b domain.Book
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&b, "id = ?", id).Error
	return b, translate(err, "book")
}

// Constant column allowlists are checked again at the persistence boundary.
func order(q *gorm.DB, p domain.Page, table string, allowed ...string) *gorm.DB {
	field := "created_at"
	for _, v := range allowed {
		if p.Sort == v {
			field = v
			break
		}
	}
	return q.Order(clause.OrderByColumn{Column: clause.Column{Table: table, Name: field}, Desc: p.Desc}).Order(clause.OrderByColumn{Column: clause.Column{Table: table, Name: "id"}, Desc: p.Desc})
}
func paginate(q *gorm.DB, p domain.Page) *gorm.DB { return q.Offset(p.Offset()).Limit(p.Size) }
func contains(s string) string {
	// Escape LIKE metacharacters; user search text has literal substring semantics.
	out := ""
	for _, r := range s {
		if r == '%' || r == '_' || r == '\\' {
			out += "\\"
		}
		out += string(r)
	}
	return "%" + out + "%"
}
