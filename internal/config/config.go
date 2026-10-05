package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env, Address, DatabaseURL, AccessSecret, Issuer, Audience          string
	AccessTTL, RefreshTTL, ConnLifetime, ConnIdleTime, ShutdownTimeout time.Duration
	MaxOpen, MaxIdle, AuthConcurrency                                  int
	LogLevel                                                           slog.Level
	Origins                                                            []string
	OTLPEndpoint                                                       string
}

func Load() (Config, error) { return Parse(os.Getenv) }
func Parse(get func(string) string) (Config, error) {
	c := Config{}
	var invalid []string
	value := func(k, d string) string {
		if v := get(k); v != "" {
			return v
		}
		return d
	}
	integer := func(k string, d, min, max int) int {
		n, e := strconv.Atoi(value(k, strconv.Itoa(d)))
		if e != nil || n < min || n > max {
			invalid = append(invalid, k)
		}
		return n
	}
	duration := func(k string, d time.Duration, min, max time.Duration) time.Duration {
		v, e := time.ParseDuration(value(k, d.String()))
		if e != nil || v < min || v > max {
			invalid = append(invalid, k)
		}
		return v
	}
	c.Env = value("APP_ENV", "development")
	if c.Env != "development" && c.Env != "production" && c.Env != "test" {
		invalid = append(invalid, "APP_ENV")
	}
	c.Address = net.JoinHostPort(value("HTTP_HOST", "0.0.0.0"), strconv.Itoa(integer("HTTP_PORT", 8080, 1, 65535)))
	c.AccessSecret = get("JWT_ACCESS_SECRET")
	if len(c.AccessSecret) < 32 {
		invalid = append(invalid, "JWT_ACCESS_SECRET (at least 32 bytes)")
	}
	c.Issuer = value("JWT_ISSUER", "library-service")
	c.Audience = value("JWT_AUDIENCE", "library-api")
	c.AccessTTL = duration("JWT_ACCESS_TTL", 15*time.Minute, time.Minute, 30*time.Minute)
	c.RefreshTTL = duration("JWT_REFRESH_TTL", 168*time.Hour, time.Hour, 90*24*time.Hour)
	c.ConnLifetime = duration("DB_CONN_MAX_LIFETIME", 30*time.Minute, time.Minute, 24*time.Hour)
	c.ConnIdleTime = duration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute, time.Second, 24*time.Hour)
	c.ShutdownTimeout = duration("SHUTDOWN_TIMEOUT", 15*time.Second, time.Second, time.Minute)
	c.MaxOpen = integer("DB_MAX_OPEN_CONNS", 25, 1, 1000)
	c.MaxIdle = integer("DB_MAX_IDLE_CONNS", 10, 0, 1000)
	if c.MaxIdle > c.MaxOpen {
		invalid = append(invalid, "DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	c.AuthConcurrency = integer("AUTH_MAX_CONCURRENCY", 4, 1, 64)
	if err := c.LogLevel.UnmarshalText([]byte(value("LOG_LEVEL", "info"))); err != nil {
		invalid = append(invalid, "LOG_LEVEL")
	}
	ssl := value("DB_SSLMODE", "disable")
	switch ssl {
	case "disable", "require", "verify-ca", "verify-full":
	default:
		invalid = append(invalid, "DB_SSLMODE")
	}
	if c.Env == "production" && ssl != "verify-full" {
		invalid = append(invalid, "DB_SSLMODE must be verify-full in production")
	}
	password := get("DB_PASSWORD")
	if password == "" {
		invalid = append(invalid, "DB_PASSWORD")
	}
	u := url.URL{Scheme: "postgres", User: url.UserPassword(value("DB_USER", "library"), password), Host: net.JoinHostPort(value("DB_HOST", "localhost"), strconv.Itoa(integer("DB_PORT", 5432, 1, 65535))), Path: "/" + value("DB_NAME", "library")}
	q := u.Query()
	q.Set("sslmode", ssl)
	q.Set("connect_timeout", "5")
	q.Set("timezone", "UTC")
	if root := get("DB_SSLROOTCERT"); root != "" {
		q.Set("sslrootcert", root)
	}
	u.RawQuery = q.Encode()
	c.DatabaseURL = u.String()
	for _, origin := range strings.Split(get("CORS_ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		v, e := url.Parse(origin)
		if e != nil || v.Host == "" || (v.Scheme != "http" && v.Scheme != "https") || v.Path != "" || v.RawQuery != "" || v.Fragment != "" || v.User != nil {
			invalid = append(invalid, "CORS_ALLOWED_ORIGINS")
		} else {
			c.Origins = append(c.Origins, origin)
		}
	}
	c.OTLPEndpoint = get("OTEL_EXPORTER_OTLP_ENDPOINT")
	if c.OTLPEndpoint != "" {
		v, e := url.Parse(c.OTLPEndpoint)
		if e != nil || v.Host == "" || (v.Scheme != "http" && v.Scheme != "https") {
			invalid = append(invalid, "OTEL_EXPORTER_OTLP_ENDPOINT")
		}
	}
	if len(invalid) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(invalid, ", "))
	}
	return c, nil
}
