package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Passwords uses fixed, bounded Argon2id parameters: 64 MiB, 3 passes, 2 lanes.
// Verification accepts only this format, preventing malicious encoded work factors.
type Passwords struct{}

func ValidatePassword(password string) bool {
	return utf8.ValidString(password) && utf8.RuneCountInString(password) >= 12 && len(password) <= 128
}
func (Passwords) Hash(password string) (string, error) {
	if !ValidatePassword(password) {
		return "", errors.New("password must be 12 characters to 128 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	enc := base64.RawStdEncoding
	return "$argon2id$v=19$m=65536,t=3,p=2$" + enc.EncodeToString(salt) + "$" + enc.EncodeToString(key), nil
}
func (Passwords) Verify(encoded, password string) bool {
	if len(password) > 128 {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=2" {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[4])
	expected, e2 := base64.RawStdEncoding.DecodeString(parts[5])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
