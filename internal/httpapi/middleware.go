package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"library-service/internal/domain"
	"library-service/internal/telemetry"
)

type Authenticator interface {
	Authenticate(context.Context, string) (domain.Principal, error)
}

const principalKey = "library.principal"

func principal(c *gin.Context) domain.Principal {
	p, _ := c.Get(principalKey)
	v, _ := p.(domain.Principal)
	return v
}
func Authenticate(auth Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 4096 {
			fail(c, domain.Unauthorized())
			return
		}
		p, err := auth.Authenticate(c.Request.Context(), parts[1])
		if err != nil {
			fail(c, err)
			return
		}
		c.Set(principalKey, p)
		c.Next()
	}
}
func Roles(roles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := domain.Allow(principal(c), roles...); err != nil {
			fail(c, err)
			return
		}
		c.Next()
	}
}
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.GetHeader("X-Request-ID"))
		if err != nil || id == uuid.Nil {
			id = uuid.New()
		}
		c.Set("request_id", id.String())
		c.Header("X-Request-ID", id.String())
		c.Next()
	}
}
func Logging(log *slog.Logger, t *telemetry.Telemetry) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		switch method {
		case "GET", "POST", "PATCH", "DELETE", "OPTIONS", "HEAD", "PUT":
		default:
			method = "OTHER"
		}
		ctx := propagation.TraceContext{}.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := t.Tracer.Start(ctx, method+" "+route, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		elapsed := time.Since(start)
		status := c.Writer.Status()
		span.SetAttributes(attribute.String("http.request.method", method), attribute.String("http.route", route), attribute.Int("http.response.status_code", status))
		if status >= 500 {
			span.SetStatus(codes.Error, "request failed")
		}
		t.Requests.WithLabelValues(route, method, strconv.Itoa(status)).Inc()
		t.Duration.WithLabelValues(route, method).Observe(elapsed.Seconds())
		attrs := []any{"request_id", c.GetString("request_id"), "method", method, "path", route, "status", status, "duration", elapsed}
		if p := principal(c); p.ID != uuid.Nil {
			attrs = append(attrs, "user_id", p.ID.String())
		}
		if sc := span.SpanContext(); sc.IsValid() {
			attrs = append(attrs, "trace_id", sc.TraceID().String())
		}
		// Never log request bodies, URL query strings, headers, raw errors, or SQL values.
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		log.Log(ctx, level, "http request", attrs...)
	}
}
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				log.ErrorContext(c.Request.Context(), "request panic recovered", "request_id", c.GetString("request_id"))
				if !c.Writer.Written() {
					fail(c, domain.E("INTERNAL_ERROR", "an internal error occurred", 500))
				} else {
					c.Abort()
				}
			}
		}()
		c.Next()
	}
}
func Limits() gin.HandlerFunc {
	return func(c *gin.Context) {
		query, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil {
			fail(c, domain.Validation(map[string]string{"query": "malformed query parameters"}))
			return
		}
		for key, values := range query {
			if len(values) != 1 {
				fail(c, domain.Validation(map[string]string{key: "must be provided only once"}))
				return
			}
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Cache-Control", "no-store")
		origin := c.GetHeader("Origin")
		if origin != "" {
			allowed := false
			for _, o := range origins {
				if o == origin {
					allowed = true
					break
				}
			}
			c.Writer.Header().Add("Vary", "Origin")
			if !allowed {
				fail(c, domain.Forbidden())
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
			if c.Request.Method == "OPTIONS" {
				c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				c.Header("Access-Control-Max-Age", "600")
				c.AbortWithStatus(204)
				return
			}
		}
		c.Next()
	}
}

// AuthWork bounds Argon2 memory/CPU. Gateways must additionally rate limit by IP/account.
func AuthWork(max int) gin.HandlerFunc {
	slots := make(chan struct{}, max)
	return func(c *gin.Context) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			c.Next()
		default:
			c.Header("Retry-After", "1")
			fail(c, domain.E("RATE_LIMITED", "authentication capacity reached; retry shortly", 429))
		}
	}
}
