package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"library-service/internal/domain"
	"library-service/internal/service"
)

func request(t *testing.T, h http.Handler, method, path, token string, body any, status int) map[string]any {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, status, w.Code, "%s %s: %s", method, path, w.Body.String())
	if status == 204 {
		return nil
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}
func dataOf(out map[string]any) map[string]any { return out["data"].(map[string]any) }
func TestCompleteHTTPFlow(t *testing.T) {
	db, s, _ := database(t)
	h, auth := application(t, db, s)
	ctx := t.Context()
	password := "correct-horse-library-secret"
	admin, err := auth.Bootstrap(ctx, service.Registration{FirstName: "Admin", LastName: "Owner", BirthYear: 1990, Email: "admin@example.com", Password: password})
	require.NoError(t, err)
	register := map[string]any{"first_name": " Ada ", "last_name": "Lovelace", "birth_year": 1990, "email": " ADA@EXAMPLE.COM ", "password": password}
	register["role"] = "ADMIN"
	request(t, h, "POST", "/api/v1/auth/register", "", register, 400)
	delete(register, "role")
	reader := dataOf(request(t, h, "POST", "/api/v1/auth/register", "", register, 201))
	require.Equal(t, "READER", reader["role"])
	require.Equal(t, "ada@example.com", reader["email"])
	require.NotContains(t, reader, "password_hash")
	readerID := reader["id"].(string)
	request(t, h, "POST", "/api/v1/auth/register", "", register, 409)
	login := func(email, pass string) map[string]any {
		return dataOf(request(t, h, "POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": pass}, 200))
	}
	a := login(admin.Email, password)
	adminToken := a["access_token"].(string)
	r := login("ada@example.com", password)
	readerToken := r["access_token"].(string)
	refresh := r["refresh_token"].(string)
	request(t, h, "POST", "/api/v1/auth/login", "", map[string]string{"email": "ada@example.com", "password": "wrong"}, 401)
	request(t, h, "GET", "/api/v1/books", "", nil, 401)
	request(t, h, "GET", "/api/v1/admin/readers", readerToken, nil, 403)
	request(t, h, "GET", "/api/v1/admin/readers", adminToken, nil, 200)
	request(t, h, "PATCH", "/api/v1/admin/readers/"+admin.ID.String(), adminToken, map[string]string{"first_name": "wrong scope"}, 404)
	request(t, h, "GET", "/api/v1/books?page=0", readerToken, nil, 422)
	request(t, h, "GET", "/api/v1/books?page_size=101", readerToken, nil, 422)
	request(t, h, "POST", "/api/v1/books", readerToken, map[string]any{"title": "Book", "author": "Author", "publication_year": 2020}, 403)
	b := dataOf(request(t, h, "POST", "/api/v1/books", adminToken, map[string]any{"title": " Clean Code ", "author": "Robert Martin", "publication_year": 2008}, 201))
	book := b["id"].(string)
	base := "/api/v1/books/" + book
	require.Equal(t, "Clean Code", b["title"])
	request(t, h, "PATCH", base, adminToken, map[string]any{"likes_count": 99}, 400)
	request(t, h, "PATCH", base, adminToken, map[string]any{"title": "Clean Code Revised"}, 200)
	request(t, h, "GET", "/api/v1/books?title=clean&author=martin&publication_year=2008&sort=likes_count", readerToken, nil, 200)
	request(t, h, "GET", base, readerToken, nil, 200)
	for i := 0; i < 2; i++ {
		request(t, h, "POST", base+"/favorite", readerToken, nil, 204)
		request(t, h, "POST", base+"/like", readerToken, nil, 204)
	}
	favorites := request(t, h, "GET", "/api/v1/me/favorites", readerToken, nil, 200)
	require.Len(t, favorites["data"], 1)
	c := dataOf(request(t, h, "POST", base+"/comments", readerToken, map[string]string{"content": " Helpful book "}, 201))
	comment := c["id"].(string)
	require.Equal(t, "Helpful book", c["content"])
	require.NotContains(t, c["reader"], "email")
	request(t, h, "PATCH", base+"/comments/"+comment, readerToken, map[string]string{"content": "Updated comment"}, 200)
	request(t, h, "GET", base+"/comments", readerToken, nil, 200)
	register["email"] = "other@example.com"
	request(t, h, "POST", "/api/v1/auth/register", "", register, 201)
	other := login("other@example.com", password)["access_token"].(string)
	request(t, h, "PATCH", base+"/comments/"+comment, other, map[string]string{"content": "hijack"}, 403)
	request(t, h, "DELETE", base+"/comments/"+comment, other, nil, 403)
	request(t, h, "PATCH", "/api/v1/me", readerToken, map[string]string{"role": "ADMIN"}, 400)
	request(t, h, "PATCH", "/api/v1/me", readerToken, map[string]string{"first_name": "Augusta"}, 200)
	request(t, h, "GET", "/api/v1/me", readerToken, nil, 200)
	request(t, h, "GET", "/api/v1/admin/readers/"+readerID, adminToken, nil, 200)
	request(t, h, "PATCH", "/api/v1/admin/readers/"+readerID, adminToken, map[string]string{"last_name": "Byron"}, 200)
	request(t, h, "PATCH", "/api/v1/admin/readers/"+readerID+"/block", adminToken, nil, 204)
	request(t, h, "GET", base, readerToken, nil, 403)
	request(t, h, "POST", base+"/like", readerToken, nil, 403)
	request(t, h, "PATCH", base+"/comments/"+comment, readerToken, map[string]string{"content": "blocked"}, 403)
	request(t, h, "DELETE", base+"/comments/"+comment, readerToken, nil, 403)
	request(t, h, "POST", "/api/v1/auth/login", "", map[string]string{"email": "ada@example.com", "password": password}, 401)
	request(t, h, "POST", "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh}, 401)
	request(t, h, "PATCH", "/api/v1/admin/readers/"+readerID+"/unblock", adminToken, nil, 204)
	request(t, h, "GET", base, readerToken, nil, 401)
	r = login("ada@example.com", password)
	refresh = r["refresh_token"].(string)
	rotated := dataOf(request(t, h, "POST", "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh}, 200))
	require.NotEqual(t, refresh, rotated["refresh_token"])
	request(t, h, "POST", "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh}, 401)
	request(t, h, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": rotated["refresh_token"]}, 401)
	r = login("ada@example.com", password)
	readerToken = r["access_token"].(string)
	refresh = r["refresh_token"].(string)
	request(t, h, "POST", "/api/v1/auth/logout", "", map[string]string{"refresh_token": refresh}, 204)
	request(t, h, "POST", "/api/v1/auth/logout", "", map[string]string{"refresh_token": refresh}, 204)
	request(t, h, "POST", "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh}, 401)
	request(t, h, "DELETE", base+"/comments/"+comment, adminToken, nil, 204)
	c = dataOf(request(t, h, "POST", base+"/comments", readerToken, map[string]string{"content": "Own comment"}, 201))
	request(t, h, "DELETE", base+"/comments/"+c["id"].(string), readerToken, nil, 204)
	for i := 0; i < 2; i++ {
		request(t, h, "DELETE", base+"/like", readerToken, nil, 204)
		request(t, h, "DELETE", base+"/favorite", readerToken, nil, 204)
	}
	current := dataOf(request(t, h, "GET", base, readerToken, nil, 200))
	require.EqualValues(t, 0, current["likes_count"])
	require.EqualValues(t, 0, current["comments_count"])
	request(t, h, "PATCH", "/api/v1/me/password", readerToken, map[string]string{"current_password": "incorrect", "new_password": "new-password-long"}, 401)
	request(t, h, "PATCH", "/api/v1/me/password", readerToken, map[string]string{"current_password": password, "new_password": "new-password-long"}, 204)
	request(t, h, "GET", "/api/v1/me", readerToken, nil, 401)
	r = login("ada@example.com", "new-password-long")
	readerToken = r["access_token"].(string)
	request(t, h, "DELETE", base, adminToken, nil, 204)
	request(t, h, "GET", base, readerToken, nil, 404)
	request(t, h, "GET", "/api/v1/books/"+uuid.NewString(), readerToken, nil, 404)
	request(t, h, "DELETE", "/api/v1/admin/readers/"+readerID, adminToken, nil, 204)
	request(t, h, "GET", "/api/v1/me", readerToken, nil, 401)
	request(t, h, "GET", "/health/live", "", nil, 200)
	request(t, h, "GET", "/health/ready", "", nil, 200)
	request(t, h, "GET", "/docs/openapi.json", "", nil, 200)
	var count int64
	require.NoError(t, db.WithContext(ctx).Model(&domain.Book{}).Count(&count).Error)
	require.Zero(t, count)
}
