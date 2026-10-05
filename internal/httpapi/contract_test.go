package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"library-service/docs"
	"library-service/internal/config"
	"library-service/internal/telemetry"
)

type healthy struct{}

func (healthy) PingContext(context.Context) error { return nil }
func TestOpenAPITracksRouter(t *testing.T) {
	tel, err := telemetry.New(t.Context(), "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	r := New(config.Config{Env: "production", AuthConcurrency: 1}, slog.New(slog.NewTextHandler(io.Discard, nil)), tel, healthy{}, nil, nil, nil, nil)
	var spec struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	require.NoError(t, json.Unmarshal(docs.OpenAPI(), &spec))
	documented := map[string]bool{}
	for path, methods := range spec.Paths {
		for method := range methods {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}
	for _, route := range r.Routes() {
		parts := strings.Split(route.Path, "/")
		for i, p := range parts {
			if strings.HasPrefix(p, ":") {
				parts[i] = "{" + p[1:] + "}"
			}
		}
		key := route.Method + " " + strings.Join(parts, "/")
		require.True(t, documented[key], "undocumented route: %s", key)
		delete(documented, key)
	}
	require.Empty(t, documented, "documented routes missing in router")
	var tree any
	require.NoError(t, json.Unmarshal(docs.OpenAPI(), &tree))
	var walk func(any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			if reference, ok := node["$ref"].(string); ok {
				prefix := "#/components/schemas/"
				require.True(t, strings.HasPrefix(reference, prefix))
				require.Contains(t, spec.Components.Schemas, strings.TrimPrefix(reference, prefix))
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(tree)
}
