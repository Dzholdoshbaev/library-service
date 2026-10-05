package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"library-service/internal/domain"
	"testing"
)

type userStub struct {
	UserRepository
	user                            domain.User
	updates, statusChanges, deletes int
}

func (s *userStub) UserByID(context.Context, uuid.UUID) (domain.User, error) { return s.user, nil }
func (s *userStub) UpdateProfile(context.Context, domain.Principal, uuid.UUID, domain.ProfilePatch) (domain.User, error) {
	s.updates++
	return s.user, nil
}
func (s *userStub) SetReaderStatus(context.Context, domain.Principal, uuid.UUID, domain.Status) error {
	s.statusChanges++
	return nil
}
func (s *userStub) DeleteReader(context.Context, domain.Principal, uuid.UUID) error {
	s.deletes++
	return nil
}
func TestUserPolicy(t *testing.T) {
	reader := domain.User{ID: uuid.New(), Role: domain.Reader, AuthVersion: 1}
	admin := domain.Principal{ID: uuid.New(), Role: domain.Admin, Version: 1}
	repo := &userStub{user: reader}
	svc := NewUsers(repo)
	require.Error(t, svc.Status(t.Context(), reader.Principal(), reader.ID, domain.Blocked))
	require.Zero(t, repo.statusChanges)
	require.NoError(t, svc.Status(t.Context(), admin, reader.ID, domain.Blocked))
	require.NoError(t, svc.Status(t.Context(), admin, reader.ID, domain.Active))
	require.Equal(t, 2, repo.statusChanges)
	require.Error(t, svc.Delete(t.Context(), reader.Principal(), reader.ID))
	require.NoError(t, svc.Delete(t.Context(), admin, reader.ID))
	require.Equal(t, 1, repo.deletes)
	empty := " "
	_, err := svc.Update(t.Context(), reader.Principal(), reader.ID, domain.ProfilePatch{FirstName: &empty})
	require.Error(t, err)
	require.Zero(t, repo.updates)
	_, err = svc.Update(t.Context(), reader.Principal(), uuid.New(), domain.ProfilePatch{})
	require.True(t, domain.HasCode(err, "FORBIDDEN"))
	name := "New name"
	_, err = svc.Update(t.Context(), reader.Principal(), reader.ID, domain.ProfilePatch{FirstName: &name})
	require.NoError(t, err)
	repo.user.Role = domain.Admin
	_, err = svc.UpdateReader(t.Context(), admin, admin.ID, domain.ProfilePatch{})
	require.True(t, domain.HasCode(err, "USER_NOT_FOUND"))
}
