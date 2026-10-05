package security

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordValidation(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		valid       bool
	}{{"short", "short", false}, {"minimum", "abcdefghijkl", true}, {"max", strings.Repeat("a", 128), true}, {"too long", strings.Repeat("a", 129), false}, {"unicode", strings.Repeat("ё", 12), true}} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.valid, ValidatePassword(tc.value)) })
	}
}
func TestArgon2(t *testing.T) {
	p := Passwords{}
	a, err := p.Hash("a-long-secret-password")
	require.NoError(t, err)
	b, err := p.Hash("a-long-secret-password")
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.True(t, p.Verify(a, "a-long-secret-password"))
	require.False(t, p.Verify(a, "wrong-password"))
	for _, v := range []string{"", "$argon2id$v=19$m=999999999,t=3,p=2$x$x", a + "$extra"} {
		require.False(t, p.Verify(v, "a-long-secret-password"))
	}
	_, err = p.Hash("short")
	require.Error(t, err)
}
