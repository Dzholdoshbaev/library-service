package httpapi

import (
	"time"

	"github.com/google/uuid"

	"library-service/internal/domain"
	"library-service/internal/service"
)

type registerRequest struct {
	FirstName  string  `json:"first_name" validate:"required,max=100"`
	LastName   string  `json:"last_name" validate:"required,max=100"`
	MiddleName *string `json:"middle_name" validate:"omitempty,max=100"`
	BirthYear  int     `json:"birth_year" validate:"required,min=1900,max=2200"`
	Email      string  `json:"email" validate:"required,email,max=254"`
	Password   string  `json:"password" validate:"required,max=128"`
}

func (r *registerRequest) normalize() { r.Email = service.NormalizeEmail(r.Email) }
func (r registerRequest) input() service.Registration {
	return service.Registration{FirstName: r.FirstName, LastName: r.LastName, MiddleName: r.MiddleName, BirthYear: r.BirthYear, Email: r.Email, Password: r.Password}
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=128"`
}

func (r *loginRequest) normalize() { r.Email = service.NormalizeEmail(r.Email) }

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,len=43"`
}
type passwordRequest struct {
	Current string `json:"current_password" validate:"required,max=128"`
	New     string `json:"new_password" validate:"required,max=128"`
}
type profileRequest struct {
	FirstName  *string `json:"first_name" validate:"omitempty,min=1,max=100"`
	LastName   *string `json:"last_name" validate:"omitempty,min=1,max=100"`
	MiddleName *string `json:"middle_name" validate:"omitempty,max=100"`
	BirthYear  *int    `json:"birth_year" validate:"omitempty,min=1900,max=2200"`
	Email      *string `json:"email" validate:"omitempty,email,max=254"`
}

func (r *profileRequest) normalize() {
	if r.Email != nil {
		v := service.NormalizeEmail(*r.Email)
		r.Email = &v
	}
}
func (r profileRequest) input() domain.ProfilePatch {
	return domain.ProfilePatch{FirstName: r.FirstName, LastName: r.LastName, MiddleName: r.MiddleName, BirthYear: r.BirthYear, Email: r.Email}
}

type createBookRequest struct {
	Title  string `json:"title" validate:"required,max=300"`
	Author string `json:"author" validate:"required,max=200"`
	Year   int    `json:"publication_year" validate:"required,min=1,max=2200"`
	Volume *int   `json:"volume" validate:"omitempty,min=1,max=2147483647"`
}
type patchBookRequest struct {
	Title  *string `json:"title" validate:"omitempty,min=1,max=300"`
	Author *string `json:"author" validate:"omitempty,min=1,max=200"`
	Year   *int    `json:"publication_year" validate:"omitempty,min=1,max=2200"`
	Volume *int    `json:"volume" validate:"omitempty,min=1,max=2147483647"`
}
type commentRequest struct {
	Content string `json:"content" validate:"required,max=4000"`
}
type userResponse struct {
	ID         uuid.UUID     `json:"id"`
	FirstName  string        `json:"first_name"`
	LastName   string        `json:"last_name"`
	MiddleName *string       `json:"middle_name"`
	BirthYear  int           `json:"birth_year"`
	Email      string        `json:"email"`
	Role       domain.Role   `json:"role"`
	Status     domain.Status `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

func userDTO(u domain.User) userResponse {
	return userResponse{u.ID, u.FirstName, u.LastName, u.MiddleName, u.BirthYear, u.Email, u.Role, u.Status, u.CreatedAt, u.UpdatedAt}
}

type bookResponse struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Author          string    `json:"author"`
	PublicationYear int       `json:"publication_year"`
	Volume          *int      `json:"volume"`
	LikesCount      int64     `json:"likes_count"`
	CommentsCount   int64     `json:"comments_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func bookDTO(b domain.Book) bookResponse {
	return bookResponse{b.ID, b.Title, b.Author, b.PublicationYear, b.Volume, b.LikesCount, b.CommentsCount, b.CreatedAt, b.UpdatedAt}
}
func booksDTO(list []domain.Book) []bookResponse {
	out := make([]bookResponse, 0, len(list))
	for _, b := range list {
		out = append(out, bookDTO(b))
	}
	return out
}

type readerResponse struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
}
type commentResponse struct {
	ID        uuid.UUID      `json:"id"`
	BookID    uuid.UUID      `json:"book_id"`
	Content   string         `json:"content"`
	Reader    readerResponse `json:"reader"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func commentDTO(c domain.Comment) commentResponse {
	r := readerResponse{c.ReaderID, c.Reader.FirstName, c.Reader.LastName}
	if c.Reader.DeletedAt != nil {
		r.FirstName = "Deleted"
		r.LastName = "reader"
	}
	return commentResponse{c.ID, c.BookID, c.Content, r, c.CreatedAt, c.UpdatedAt}
}

type credentialsResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"`
	User         userResponse `json:"user"`
}

func credentialsDTO(c service.Credentials) credentialsResponse {
	return credentialsResponse{c.AccessToken, c.RefreshToken, "Bearer", c.ExpiresIn, userDTO(c.User)}
}
