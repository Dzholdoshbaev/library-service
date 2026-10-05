package httpapi

import "github.com/gin-gonic/gin"

func (a *API) register(c *gin.Context) {
	var in registerRequest
	if !bind(c, a.validate, &in) {
		return
	}
	u, err := a.auth.Register(c.Request.Context(), in.input())
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 201, userDTO(u))
}
func (a *API) login(c *gin.Context) {
	var in loginRequest
	if !bind(c, a.validate, &in) {
		return
	}
	out, err := a.auth.Login(c.Request.Context(), in.Email, in.Password)
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, credentialsDTO(out))
}
func (a *API) refresh(c *gin.Context) {
	var in refreshRequest
	if !bind(c, a.validate, &in) {
		return
	}
	out, err := a.auth.Refresh(c.Request.Context(), in.RefreshToken)
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, credentialsDTO(out))
}
func (a *API) logout(c *gin.Context) {
	var in refreshRequest
	if !bind(c, a.validate, &in) {
		return
	}
	if err := a.auth.Logout(c.Request.Context(), in.RefreshToken); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (a *API) password(c *gin.Context) {
	var in passwordRequest
	if !bind(c, a.validate, &in) {
		return
	}
	if err := a.auth.Password(c.Request.Context(), principal(c), in.Current, in.New); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
