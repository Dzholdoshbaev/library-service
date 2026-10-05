package httpapi

import (
	"github.com/gin-gonic/gin"

	"library-service/internal/domain"
)

func (a *API) me(c *gin.Context) {
	u, err := a.users.Me(c.Request.Context(), principal(c))
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, userDTO(u))
}
func (a *API) patchMe(c *gin.Context) {
	var in profileRequest
	if !bind(c, a.validate, &in) {
		return
	}
	p := principal(c)
	u, err := a.users.Update(c.Request.Context(), p, p.ID, in.input())
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, userDTO(u))
}
func (a *API) reader(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	u, err := a.users.Reader(c.Request.Context(), principal(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, userDTO(u))
}
func (a *API) patchReader(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var in profileRequest
	if !bind(c, a.validate, &in) {
		return
	}
	u, err := a.users.UpdateReader(c.Request.Context(), principal(c), id, in.input())
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, userDTO(u))
}
func (a *API) deleteReader(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := a.users.Delete(c.Request.Context(), principal(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (a *API) status(status domain.Status) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathID(c, "id")
		if !ok {
			return
		}
		if err := a.users.Status(c.Request.Context(), principal(c), id, status); err != nil {
			fail(c, err)
			return
		}
		c.Status(204)
	}
}
func (a *API) readers(c *gin.Context) {
	p, ok := page(c, "created_at", "first_name", "last_name", "email", "birth_year")
	if !ok {
		return
	}
	f := domain.UserFilter{Page: p, Status: domain.Status(c.Query("status"))}
	if f.Status != "" && f.Status != domain.Active && f.Status != domain.Blocked {
		fail(c, domain.Validation(map[string]string{"status": "must be ACTIVE or BLOCKED"}))
		return
	}
	if f.FirstName, ok = queryText(c, "first_name", 100); !ok {
		return
	}
	if f.LastName, ok = queryText(c, "last_name", 100); !ok {
		return
	}
	if f.Email, ok = queryText(c, "email", 254); !ok {
		return
	}
	users, total, err := a.users.List(c.Request.Context(), principal(c), f)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		out = append(out, userDTO(u))
	}
	collection(c, out, total, p)
}
