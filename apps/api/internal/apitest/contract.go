package apitest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

var (
	specOnce   sync.Once
	specRouter routers.Router
	specErr    error
)

// SpecPath returns the absolute path of docs/openapi.yaml.
func SpecPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "docs", "openapi.yaml")
}

// loadSpec parses the OpenAPI document once and makes it strict: object schemas that do not
// say otherwise reject properties they do not declare, so a handler that starts returning an
// undocumented field fails the tests until the spec is updated.
func loadSpec() (routers.Router, error) {
	specOnce.Do(func() {
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile(SpecPath())
		if err != nil {
			specErr = fmt.Errorf("load %s: %w", SpecPath(), err)
			return
		}
		if err := doc.Validate(context.Background()); err != nil {
			specErr = fmt.Errorf("invalid OpenAPI document: %w", err)
			return
		}
		seen := map[*openapi3.Schema]bool{}
		for _, ref := range doc.Components.Schemas {
			forbidUndeclaredProperties(ref, seen)
		}
		// Match any host: tests run on a random local port.
		doc.Servers = openapi3.Servers{{URL: "/"}}
		specRouter, specErr = gorillamux.NewRouter(doc)
	})
	return specRouter, specErr
}

func forbidUndeclaredProperties(ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) {
	if ref == nil || ref.Value == nil || seen[ref.Value] {
		return
	}
	s := ref.Value
	seen[s] = true
	if len(s.Properties) > 0 && s.AdditionalProperties.Has == nil && s.AdditionalProperties.Schema == nil {
		no := false
		s.AdditionalProperties.Has = &no
	}
	for _, p := range s.Properties {
		forbidUndeclaredProperties(p, seen)
	}
	forbidUndeclaredProperties(s.Items, seen)
	for _, list := range []openapi3.SchemaRefs{s.AllOf, s.AnyOf, s.OneOf} {
		for _, sub := range list {
			forbidUndeclaredProperties(sub, seen)
		}
	}
}

// contractMiddleware checks that every request matches a documented operation and that every
// response has a documented status code and a body matching its schema. Violations fail the test.
func contractMiddleware(t *testing.T, next http.Handler) http.Handler {
	router, err := loadSpec()
	if err != nil {
		t.Fatalf("OpenAPI contract: %v", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(reqBody))

		route, pathParams, err := router.FindRoute(r)
		if err != nil {
			t.Errorf("OpenAPI contract: %s %s is not documented: %v", r.Method, r.URL.Path, err)
			next.ServeHTTP(w, r)
			return
		}

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)

		input := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
			},
			Status:  rec.Code,
			Header:  rec.Header(),
			Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
		}
		input.SetBodyBytes(rec.Body.Bytes())
		if err := openapi3filter.ValidateResponse(r.Context(), input); err != nil {
			t.Errorf("OpenAPI contract: %s %s -> %d does not match docs/openapi.yaml: %v\nbody: %s",
				r.Method, r.URL.Path, rec.Code, err, rec.Body.Bytes())
		}

		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}
