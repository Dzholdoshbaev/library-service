package domain

import "errors"

type Error struct {
	Code    string
	Message string
	Status  int
	Fields  map[string]string
	Cause   error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }
func E(code, message string, status int) *Error {
	return &Error{Code: code, Message: message, Status: status}
}
func Validation(fields map[string]string) *Error {
	return &Error{Code: "VALIDATION_ERROR", Message: "request validation failed", Status: 422, Fields: fields}
}
func Unauthorized() *Error { return E("UNAUTHORIZED", "authentication required or invalid", 401) }
func Forbidden() *Error    { return E("FORBIDDEN", "permission denied", 403) }
func Credentials() *Error  { return E("INVALID_CREDENTIALS", "invalid email or password", 401) }
func UserBlocked() *Error  { return E("USER_BLOCKED", "account is blocked", 403) }
func NotFound(kind string) *Error {
	switch kind {
	case "user":
		return E("USER_NOT_FOUND", "user not found", 404)
	case "book":
		return E("BOOK_NOT_FOUND", "book not found", 404)
	default:
		return E("COMMENT_NOT_FOUND", "comment not found", 404)
	}
}
func HasCode(err error, code string) bool { var e *Error; return errors.As(err, &e) && e.Code == code }
func Allow(p Principal, roles ...Role) error {
	for _, r := range roles {
		if p.Role == r {
			return nil
		}
	}
	return Forbidden()
}
