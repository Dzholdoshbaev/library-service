// Package docs embeds the API contract and development documentation page.
package docs

import _ "embed"

//go:embed openapi.json
var openAPI string

//go:embed swagger.html
var swagger string

// OpenAPI returns an independent copy of the embedded contract.
func OpenAPI() []byte { return []byte(openAPI) }

// Swagger returns an independent copy of the development UI document.
func Swagger() []byte { return []byte(swagger) }
