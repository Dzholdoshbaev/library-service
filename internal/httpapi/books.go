package httpapi

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"library-service/internal/domain"
)

func (a *API) createBook(c *gin.Context) {
	var in createBookRequest
	if !bind(c, a.validate, &in) {
		return
	}
	out, err := a.books.Create(c.Request.Context(), principal(c), domain.Book{Title: in.Title, Author: in.Author, PublicationYear: in.Year, Volume: in.Volume})
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Location", "/api/v1/books/"+out.ID.String())
	data(c, 201, bookDTO(out))
}
func (a *API) getBook(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	b, err := a.books.Get(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, bookDTO(b))
}
func (a *API) patchBook(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var in patchBookRequest
	if !bind(c, a.validate, &in) {
		return
	}
	b, err := a.books.Update(c.Request.Context(), principal(c), id, domain.BookPatch{Title: in.Title, Author: in.Author, PublicationYear: in.Year, Volume: in.Volume})
	if err != nil {
		fail(c, err)
		return
	}
	data(c, 200, bookDTO(b))
}
func (a *API) deleteBook(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := a.books.Delete(c.Request.Context(), principal(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(204)
}
func (a *API) listBooks(c *gin.Context) {
	p, ok := page(c, "created_at", "title", "author", "publication_year", "likes_count", "comments_count")
	if !ok {
		return
	}
	f := domain.BookFilter{Page: p}
	if f.Title, ok = queryText(c, "title", 300); !ok {
		return
	}
	if f.Author, ok = queryText(c, "author", 200); !ok {
		return
	}
	if raw := c.Query("publication_year"); c.Request.URL.Query().Has("publication_year") {
		year, err := strconv.Atoi(raw)
		if err != nil || year < 1 || year > time.Now().UTC().Year()+1 {
			fail(c, domain.Validation(map[string]string{"publication_year": "must be between 1 and next year"}))
			return
		}
		f.Year = year
	}
	books, total, err := a.books.List(c.Request.Context(), f)
	if err != nil {
		fail(c, err)
		return
	}
	collection(c, booksDTO(books), total, p)
}
func (a *API) favorite(on bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathID(c, "id")
		if !ok {
			return
		}
		if err := a.books.Favorite(c.Request.Context(), principal(c), id, on); err != nil {
			fail(c, err)
			return
		}
		c.Status(204)
	}
}
func (a *API) like(on bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathID(c, "id")
		if !ok {
			return
		}
		if err := a.books.Like(c.Request.Context(), principal(c), id, on); err != nil {
			fail(c, err)
			return
		}
		c.Status(204)
	}
}
func (a *API) favorites(c *gin.Context) {
	p, ok := page(c, "created_at", "title", "author", "publication_year", "likes_count", "comments_count")
	if !ok {
		return
	}
	books, total, err := a.books.Favorites(c.Request.Context(), principal(c), p)
	if err != nil {
		fail(c, err)
		return
	}
	collection(c, booksDTO(books), total, p)
}
