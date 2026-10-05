package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"library-service/internal/domain"
)

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string { return strings.Split(f.Tag.Get("json"), ",")[0] })
	return v
}
func fail(c *gin.Context, err error) {
	var e *domain.Error
	if !errors.As(err, &e) {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			e = domain.E("REQUEST_TIMEOUT", "request timed out", 504)
		case errors.Is(err, context.Canceled):
			e = domain.E("REQUEST_CANCELED", "request canceled", 408)
		default:
			e = domain.E("INTERNAL_ERROR", "an internal error occurred", 500)
		}
	}
	_ = c.Error(err)
	body := gin.H{"code": e.Code, "message": e.Message}
	if len(e.Fields) > 0 {
		body["fields"] = e.Fields
	}
	if e.Status == 401 {
		c.Header("WWW-Authenticate", "Bearer")
	}
	c.AbortWithStatusJSON(e.Status, gin.H{"error": body})
}
func data(c *gin.Context, status int, v any) { c.JSON(status, gin.H{"data": v}) }
func collection(c *gin.Context, v any, total int64, p domain.Page) {
	pages := (total + int64(p.Size) - 1) / int64(p.Size)
	c.JSON(200, gin.H{"data": v, "pagination": gin.H{"page": p.Number, "page_size": p.Size, "total": total, "total_pages": pages}})
}
func bind(c *gin.Context, v *validator.Validate, dst any) bool {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		fail(c, domain.E("UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json", 415))
		return false
	}
	dec := json.NewDecoder(c.Request.Body)
	var raw json.RawMessage
	if err = dec.Decode(&raw); err != nil {
		invalidJSON(c, err)
		return false
	}
	if err = dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		invalidJSON(c, err)
		return false
	}
	if len(raw) == 0 || raw[0] != '{' || !utf8.Valid(raw) {
		invalidJSON(c, nil)
		return false
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err = strict.Decode(dst); err != nil {
		invalidJSON(c, err)
		return false
	}
	if n, ok := dst.(interface{ normalize() }); ok {
		n.normalize()
	}
	if err = v.Struct(dst); err != nil {
		var violations validator.ValidationErrors
		if !errors.As(err, &violations) {
			fail(c, err)
			return false
		}
		fields := map[string]string{}
		for _, e := range violations {
			message := "invalid value"
			switch e.Tag() {
			case "required":
				message = "is required"
			case "email":
				message = "must be a valid email"
			case "min":
				message = "must meet minimum " + e.Param()
			case "max":
				message = "must not exceed " + e.Param()
			case "len":
				message = "must have length " + e.Param()
			}
			fields[e.Field()] = message
		}
		fail(c, domain.Validation(fields))
		return false
	}
	return true
}

func invalidJSON(c *gin.Context, err error) {
	var max *http.MaxBytesError
	if errors.As(err, &max) {
		fail(c, domain.E("BODY_TOO_LARGE", "request body exceeds 1 MiB", 413))
		return
	}
	fail(c, domain.E("INVALID_JSON", "expected one UTF-8 JSON object with only supported fields", 400))
}
func pathID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil || id == uuid.Nil {
		fail(c, domain.Validation(map[string]string{name: "must be a nonzero UUID"}))
		return uuid.Nil, false
	}
	return id, true
}
func page(c *gin.Context, sorts ...string) (domain.Page, bool) {
	p := domain.Page{Number: 1, Size: 20, Sort: "created_at", Desc: true}
	for k, target := range map[string]*int{"page": &p.Number, "page_size": &p.Size} {
		if raw := c.Query(k); c.Request.URL.Query().Has(k) {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || (k == "page_size" && n > 100) || (k == "page" && n > 1000000) {
				fail(c, domain.Validation(map[string]string{k: "must be a positive integer within the allowed range"}))
				return p, false
			}
			*target = n
		}
	}
	if s := c.Query("sort"); c.Request.URL.Query().Has("sort") {
		valid := false
		for _, a := range sorts {
			if s == a {
				valid = true
			}
		}
		if !valid {
			fail(c, domain.Validation(map[string]string{"sort": "unsupported sort field"}))
			return p, false
		}
		p.Sort = s
	}
	if o := c.Query("order"); c.Request.URL.Query().Has("order") {
		if o != "asc" && o != "desc" {
			fail(c, domain.Validation(map[string]string{"order": "must be asc or desc"}))
			return p, false
		}
		p.Desc = o == "desc"
	}
	return p, true
}
func queryText(c *gin.Context, name string, max int) (string, bool) {
	v := strings.TrimSpace(c.Query(name))
	if len(v) > max {
		fail(c, domain.Validation(map[string]string{name: "search value is too long"}))
		return "", false
	}
	return v, true
}
