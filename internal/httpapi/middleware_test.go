package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"library-service/internal/domain"
	"library-service/internal/telemetry"
)

type authStub struct {
	p   domain.Principal
	err error
}

func (s authStub) Authenticate(context.Context, string) (domain.Principal, error) { return s.p, s.err }
func TestAuthAndRBAC(t *testing.T) {
	for _, tc := range []struct {
		name, header string
		role         domain.Role
		err          error
		status       int
	}{{"missing", "", domain.Reader, nil, 401}, {"reader", "Bearer token", domain.Reader, nil, 403}, {"admin", "Bearer token", domain.Admin, nil, 204}, {"blocked", "Bearer token", domain.Reader, domain.UserBlocked(), 403}, {"invalid", "Bearer token", domain.Reader, domain.Unauthorized(), 401}} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/admin", Authenticate(authStub{domain.Principal{ID: uuid.New(), Role: tc.role, Version: 1}, tc.err}), Roles(domain.Admin), func(c *gin.Context) { c.Status(204) })
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/admin", nil)
			req.Header.Set("Authorization", tc.header)
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code)
		})
	}
}
func TestStrictJSONAndPagination(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		status           int
	}{{"role injection", "/register", `{"first_name":"Ada","last_name":"L","birth_year":1990,"email":"a@b.com","password":"long-password","role":"ADMIN"}`, 400}, {"extra JSON", "/register", `{} {}`, 400}, {"root null", "/register", `null`, 400}, {"root array", "/register", `[]`, 400}, {"validation", "/register", `{"email":"bad"}`, 422}, {"oversize", "/register", `{"first_name":"` + strings.Repeat("x", 1<<20) + `"}`, 413}, {"bad page", "/page?page=0", "", 422}, {"empty page", "/page?page=", "", 422}, {"empty sort", "/page?sort=", "", 422}, {"repeated page", "/page?page=1&page=2", "", 422}, {"bad size", "/page?page_size=101", "", 422}, {"sort injection", "/page?sort=title;DROP%20TABLE%20books", "", 422}, {"valid pagination", "/page?page=2&page_size=10&sort=title&order=asc", "", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(Limits())
			r.POST("/register", func(c *gin.Context) {
				var in registerRequest
				if bind(c, newValidator(), &in) {
					c.Status(201)
				}
			})
			r.GET("/page", func(c *gin.Context) {
				p, ok := page(c, "title", "created_at")
				if ok {
					collection(c, []string{}, 0, p)
				}
			})
			method := "POST"
			if strings.HasPrefix(tc.path, "/page") {
				method = "GET"
			}
			req := httptest.NewRequest(method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
		})
	}
}
func TestLogsRecoveryAndCORS(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	tel, err := telemetry.New(t.Context(), "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	r := gin.New()
	r.Use(RequestID(), Logging(log, tel), Recovery(log), CORS([]string{"https://allowed.example"}))
	r.POST("/panic", func(_ *gin.Context) { panic("secret-in-panic") })
	req := httptest.NewRequest("POST", "/panic?token=secret-in-query", strings.NewReader(`{"password":"secret-in-body"}`))
	req.Header.Set("Authorization", "Bearer secret-in-header")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 500, w.Code)
	_, err = uuid.Parse(w.Header().Get("X-Request-ID"))
	require.NoError(t, err)
	for _, secret := range []string{"secret-in-panic", "secret-in-query", "secret-in-body", "secret-in-header"} {
		require.NotContains(t, logs.String(), secret)
		require.NotContains(t, w.Body.String(), secret)
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Contains(t, body, "error")
	req = httptest.NewRequest("OPTIONS", "/panic", nil)
	req.Header.Set("Origin", "https://allowed.example")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 204, w.Code)
	require.Equal(t, "https://allowed.example", w.Header().Get("Access-Control-Allow-Origin"))
	req = httptest.NewRequest("POST", "/panic", nil)
	req.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
}
func TestSafeResponses(t *testing.T) {
	encoded, err := json.Marshal(userDTO(domain.User{PasswordHash: "NEVER-RETURN", AuthVersion: 9}))
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "NEVER-RETURN")
	require.NotContains(t, string(encoded), "auth_version")
}
func TestErrorRedaction(t *testing.T) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) { fail(c, io.ErrUnexpectedEOF) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 500, w.Code)
	require.NotContains(t, w.Body.String(), io.ErrUnexpectedEOF.Error())
}
