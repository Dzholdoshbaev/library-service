package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string
type Status string

const (
	Admin   Role   = "ADMIN"
	Reader  Role   = "READER"
	Active  Status = "ACTIVE"
	Blocked Status = "BLOCKED"
)

// Entities are internal persistence/domain data, never HTTP response objects.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	FirstName    string
	LastName     string
	MiddleName   *string
	BirthYear    int
	Email        string
	PasswordHash string
	Role         Role
	Status       Status
	AuthVersion  int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}
type Book struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Title           string
	Author          string
	PublicationYear int
	Volume          *int
	LikesCount      int64
	CommentsCount   int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
type Comment struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookID    uuid.UUID `gorm:"type:uuid"`
	ReaderID  uuid.UUID `gorm:"type:uuid"`
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Reader    User `gorm:"foreignKey:ReaderID"`
}
type RefreshToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid"`
	FamilyID   uuid.UUID `gorm:"type:uuid"`
	TokenHash  string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}
type Favorite struct {
	ReaderID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
}

func (Favorite) TableName() string { return "reader_favorites" }

type Like struct {
	ReaderID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
}

func (Like) TableName() string { return "book_likes" }

type Principal struct {
	ID      uuid.UUID
	Role    Role
	Version int64
}

func (u User) Principal() Principal { return Principal{u.ID, u.Role, u.AuthVersion} }

type Page struct {
	Number int
	Size   int
	Sort   string
	Desc   bool
}

func (p Page) Offset() int { return (p.Number - 1) * p.Size }

type UserFilter struct {
	Page
	Status                     Status
	FirstName, LastName, Email string
}
type BookFilter struct {
	Page
	Title, Author string
	Year          int
}
type ProfilePatch struct {
	FirstName, LastName, MiddleName *string
	BirthYear                       *int
	Email                           *string
}
type BookPatch struct {
	Title, Author           *string
	PublicationYear, Volume *int
}
