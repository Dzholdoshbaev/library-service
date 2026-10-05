package httpapi

import "github.com/gin-gonic/gin"

func (a *API) createComment(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var in commentRequest
	if !bind(c, a.validate, &in) {
		return
	}
	comment, err := a.comments.Create(c.Request.Context(), principal(c), id, in.Content)
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 201, commentDTO(comment))
}
func (a *API) listComments(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	p, ok := page(c, "created_at", "updated_at")
	if !ok {
		return
	}
	comments, total, err := a.comments.List(c.Request.Context(), id, p)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]commentResponse, 0, len(comments))
	for _, v := range comments {
		out = append(out, commentDTO(v))
	}
	collection(c, out, total, p)
}
func (a *API) patchComment(c *gin.Context) {
	book, ok := pathID(c, "id")
	if !ok {
		return
	}
	id, ok := pathID(c, "commentId")
	if !ok {
		return
	}
	var in commentRequest
	if !bind(c, a.validate, &in) {
		return
	}
	comment, err := a.comments.Update(c.Request.Context(), principal(c), book, id, in.Content)
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, commentDTO(comment))
}
func (a *API) deleteComment(c *gin.Context) {
	book, ok := pathID(c, "id")
	if !ok {
		return
	}
	id, ok := pathID(c, "commentId")
	if !ok {
		return
	}
	if err := a.comments.Delete(c.Request.Context(), principal(c), book, id); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
