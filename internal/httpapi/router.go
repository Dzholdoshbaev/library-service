package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"library-service/docs"
	"library-service/internal/config"
	"library-service/internal/domain"
	"library-service/internal/service"
	"library-service/internal/telemetry"
)

type Pinger interface{ PingContext(context.Context) error }
type API struct {
	auth     *service.Auth
	users    *service.Users
	books    *service.Books
	comments *service.Comments
	validate *validator.Validate
}

func New(c config.Config, log *slog.Logger, t *telemetry.Telemetry, db Pinger, auth *service.Auth, users *service.Users, books *service.Books, comments *service.Comments) *gin.Engine {
	a := &API{auth, users, books, comments, newValidator()}
	r := gin.New()
	r.HandleMethodNotAllowed = true
	// No proxy is trusted implicitly. Client IP based controls belong at the edge.
	_ = r.SetTrustedProxies(nil)
	r.Use(RequestID(), Logging(log, t), Recovery(log), Limits(), CORS(c.Origins))
	r.GET("/health/live", func(c *gin.Context) { data(c, 200, gin.H{"status": "alive"}) })
	r.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			fail(c, domain.E("NOT_READY", "service is not ready", 503))
			return
		}
		data(c, 200, gin.H{"status": "ready"})
	})
	r.GET("/metrics", Authenticate(auth), Roles(domain.Admin), gin.WrapH(promhttp.HandlerFor(t.Registry, promhttp.HandlerOpts{})))
	if c.Env != "production" {
		r.GET("/docs/openapi.json", func(c *gin.Context) { c.Data(200, "application/json", docs.OpenAPI()) })
		r.GET("/docs", func(c *gin.Context) { c.Data(200, "text/html; charset=utf-8", docs.Swagger()) })
	}
	v := r.Group("/api/v1")
	authWork := AuthWork(c.AuthConcurrency)
	v.POST("/auth/register", authWork, a.register)
	v.POST("/auth/login", authWork, a.login)
	v.POST("/auth/refresh", a.refresh)
	v.POST("/auth/logout", a.logout)
	protected := v.Group("", Authenticate(auth))
	protected.GET("/me", a.me)
	protected.PATCH("/me", a.patchMe)
	protected.PATCH("/me/password", authWork, a.password)
	protected.GET("/books", a.listBooks)
	protected.GET("/books/:id", a.getBook)
	protected.POST("/books", Roles(domain.Admin), a.createBook)
	protected.PATCH("/books/:id", Roles(domain.Admin), a.patchBook)
	protected.DELETE("/books/:id", Roles(domain.Admin), a.deleteBook)
	protected.GET("/me/favorites", Roles(domain.Reader), a.favorites)
	protected.POST("/books/:id/favorite", Roles(domain.Reader), a.favorite(true))
	protected.DELETE("/books/:id/favorite", Roles(domain.Reader), a.favorite(false))
	protected.POST("/books/:id/like", Roles(domain.Reader), a.like(true))
	protected.DELETE("/books/:id/like", Roles(domain.Reader), a.like(false))
	// Consistent :id parameter avoids conflicting Gin radix-tree wildcard names.
	protected.GET("/books/:id/comments", a.listComments)
	protected.POST("/books/:id/comments", Roles(domain.Reader), a.createComment)
	protected.PATCH("/books/:id/comments/:commentId", Roles(domain.Reader), a.patchComment)
	protected.DELETE("/books/:id/comments/:commentId", Roles(domain.Reader, domain.Admin), a.deleteComment)
	admin := protected.Group("/admin", Roles(domain.Admin))
	admin.GET("/readers", a.readers)
	admin.GET("/readers/:id", a.reader)
	admin.PATCH("/readers/:id", a.patchReader)
	admin.DELETE("/readers/:id", a.deleteReader)
	admin.PATCH("/readers/:id/block", a.status(domain.Blocked))
	admin.PATCH("/readers/:id/unblock", a.status(domain.Active))
	r.NoRoute(func(c *gin.Context) { fail(c, domain.E("NOT_FOUND", "route not found", 404)) })
	r.NoMethod(func(c *gin.Context) {
		fail(c, domain.E("METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed))
	})
	return r
}
