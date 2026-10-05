package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		ok               bool
	}{{"valid", "", "", true}, {"weak key", "JWT_ACCESS_SECRET", "short", false}, {"bad pool", "DB_MAX_IDLE_CONNS", "26", false}, {"negative ttl", "JWT_ACCESS_TTL", "-1m", false}, {"bad port", "HTTP_PORT", "zero", false}, {"wildcard cors", "CORS_ALLOWED_ORIGINS", "*", false}, {"production TLS", "APP_ENV", "production", false}, {"bad level", "LOG_LEVEL", "verbose", false}} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{"JWT_ACCESS_SECRET": strings.Repeat("s", 32), "DB_PASSWORD": "test-config-password"}
			values[tc.key] = tc.value
			c, err := Parse(func(k string) string { return values[k] })
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, 25, c.MaxOpen)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "test-config-password")
			}
		})
	}
}
