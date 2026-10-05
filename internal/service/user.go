package service

import (
	"context"

	"github.com/google/uuid"

	"library-service/internal/domain"
)

type UserRepository interface {
	UserByID(context.Context, uuid.UUID) (domain.User, error)
	ListReaders(context.Context, domain.UserFilter) ([]domain.User, int64, error)
	UpdateProfile(context.Context, domain.Principal, uuid.UUID, domain.ProfilePatch) (domain.User, error)
	SetReaderStatus(context.Context, domain.Principal, uuid.UUID, domain.Status) error
	DeleteReader(context.Context, domain.Principal, uuid.UUID) error
}
type Users struct{ repo UserRepository }

func NewUsers(repo UserRepository) *Users { return &Users{repo} }
func (s *Users) Me(ctx context.Context, p domain.Principal) (domain.User, error) {
	return s.repo.UserByID(ctx, p.ID)
}
func (s *Users) Reader(ctx context.Context, p domain.Principal, id uuid.UUID) (domain.User, error) {
	if err := domain.Allow(p, domain.Admin); err != nil {
		return domain.User{}, err
	}
	u, err := s.repo.UserByID(ctx, id)
	if err != nil {
		return u, err
	}
	if u.Role != domain.Reader {
		return domain.User{}, domain.NotFound("user")
	}
	return u, nil
}
func (s *Users) List(ctx context.Context, p domain.Principal, f domain.UserFilter) ([]domain.User, int64, error) {
	if err := domain.Allow(p, domain.Admin); err != nil {
		return nil, 0, err
	}
	return s.repo.ListReaders(ctx, f)
}
func (s *Users) Update(ctx context.Context, p domain.Principal, id uuid.UUID, patch domain.ProfilePatch) (domain.User, error) {
	if id != p.ID {
		if _, err := s.Reader(ctx, p, id); err != nil {
			return domain.User{}, err
		}
	}
	if err := validateProfile(patch); err != nil {
		return domain.User{}, err
	}
	return s.repo.UpdateProfile(ctx, p, id, patch)
}
func (s *Users) Status(ctx context.Context, p domain.Principal, id uuid.UUID, status domain.Status) error {
	if err := domain.Allow(p, domain.Admin); err != nil {
		return err
	}
	if status != domain.Active && status != domain.Blocked {
		return domain.Validation(map[string]string{"status": "invalid status"})
	}
	return s.repo.SetReaderStatus(ctx, p, id, status)
}
func (s *Users) Delete(ctx context.Context, p domain.Principal, id uuid.UUID) error {
	if err := domain.Allow(p, domain.Admin); err != nil {
		return err
	}
	return s.repo.DeleteReader(ctx, p, id)
}

// UpdateReader enforces the reader administration scope even for the actor's ID.
func (s *Users) UpdateReader(ctx context.Context, p domain.Principal, id uuid.UUID, patch domain.ProfilePatch) (domain.User, error) {
	if _, err := s.Reader(ctx, p, id); err != nil {
		return domain.User{}, err
	}
	if err := validateProfile(patch); err != nil {
		return domain.User{}, err
	}
	return s.repo.UpdateProfile(ctx, p, id, patch)
}
