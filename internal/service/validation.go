package service

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"library-service/internal/domain"
	"library-service/internal/security"
)

func bounded(s string, max int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(strings.TrimSpace(s)) > 0 && utf8.RuneCountInString(s) <= max
}
func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}
func validEmail(s string) bool {
	s = NormalizeEmail(s)
	a, e := mail.ParseAddress(s)
	return e == nil && a.Address == s && len(s) <= 254
}
func validYear(y int) bool { return y >= 1900 && y <= time.Now().UTC().Year() }
func validateRegistration(in Registration) error {
	f := map[string]string{}
	if !bounded(in.FirstName, 100) {
		f["first_name"] = "must contain 1 to 100 characters"
	}
	if !bounded(in.LastName, 100) {
		f["last_name"] = "must contain 1 to 100 characters"
	}
	if in.MiddleName != nil && utf8.RuneCountInString(*in.MiddleName) > 100 {
		f["middle_name"] = "must contain at most 100 characters"
	}
	if !validYear(in.BirthYear) {
		f["birth_year"] = "must be between 1900 and the current year"
	}
	if !validEmail(in.Email) {
		f["email"] = "must be a valid email"
	}
	if !security.ValidatePassword(in.Password) {
		f["password"] = "must contain at least 12 characters and at most 128 bytes"
	}
	if len(f) > 0 {
		return domain.Validation(f)
	}
	return nil
}
func validateProfile(p domain.ProfilePatch) error {
	f := map[string]string{}
	if p.FirstName != nil && !bounded(*p.FirstName, 100) {
		f["first_name"] = "must contain 1 to 100 characters"
	}
	if p.LastName != nil && !bounded(*p.LastName, 100) {
		f["last_name"] = "must contain 1 to 100 characters"
	}
	if p.MiddleName != nil && utf8.RuneCountInString(*p.MiddleName) > 100 {
		f["middle_name"] = "must contain at most 100 characters"
	}
	if p.Email != nil && !validEmail(*p.Email) {
		f["email"] = "must be a valid email"
	}
	if p.BirthYear != nil && !validYear(*p.BirthYear) {
		f["birth_year"] = "must be between 1900 and the current year"
	}
	if len(f) > 0 {
		return domain.Validation(f)
	}
	return nil
}
func validateBook(p domain.BookPatch) error {
	f := map[string]string{}
	if p.Title != nil && !bounded(*p.Title, 300) {
		f["title"] = "must contain 1 to 300 characters"
	}
	if p.Author != nil && !bounded(*p.Author, 200) {
		f["author"] = "must contain 1 to 200 characters"
	}
	if p.PublicationYear != nil && (*p.PublicationYear < 1 || *p.PublicationYear > time.Now().UTC().Year()+1) {
		f["publication_year"] = "must be between 1 and next year"
	}
	if p.Volume != nil && (*p.Volume < 1 || *p.Volume > 2147483647) {
		f["volume"] = "must be a positive 32-bit integer"
	}
	if len(f) > 0 {
		return domain.Validation(f)
	}
	return nil
}
