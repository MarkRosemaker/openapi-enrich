package enrich

import (
	"net/http"
	"testing"

	"github.com/MarkRosemaker/openapi"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestBuildResponse_JSON(t *testing.T) {
	resp := &cassette.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": {"application/json"}},
		Body:       []byte(`{"id":1,"name":"Alice"}`),
	}

	r, err := buildResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if r.Description != "OK" {
		t.Errorf("description: got %q, want OK", r.Description)
	}
	if len(r.Content) == 0 {
		t.Fatal("expected content")
	}
	mt := r.Content["application/json"]
	if mt == nil {
		t.Fatal("expected application/json content")
	}
	if mt.Schema == nil || mt.Schema.Value == nil {
		t.Fatal("expected schema")
	}
	if mt.Schema.Value.Type != openapi.TypeObject {
		t.Errorf("schema type: got %q, want object", mt.Schema.Value.Type)
	}
}

func TestBuildResponse_NoBody(t *testing.T) {
	resp := &cassette.Response{
		StatusCode: 204,
		Headers:    http.Header{},
		Body:       nil,
	}

	r, err := buildResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if r.Description != "No Content" {
		t.Errorf("description: got %q, want No Content", r.Description)
	}
	if len(r.Content) != 0 {
		t.Errorf("expected no content for 204 with no body, got %d entries", len(r.Content))
	}
}

func TestBuildResponse_TextPlain(t *testing.T) {
	resp := &cassette.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": {"text/plain"}},
		Body:       []byte("Hello, World!"),
	}

	r, err := buildResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Content) == 0 {
		t.Fatal("expected content for text/plain")
	}
	mt := r.Content["text/plain"]
	if mt == nil || mt.Schema == nil {
		t.Fatal("expected text/plain schema")
	}
	if mt.Schema.Value.Type != openapi.TypeString {
		t.Errorf("schema type: got %q, want string", mt.Schema.Value.Type)
	}
}

func TestBuildResponse_TextPlain_SameAsStatus(t *testing.T) {
	resp := &cassette.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": {"text/plain"}},
		Body:       []byte("OK"),
	}

	r, err := buildResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	// When body == status text, no schema
	if len(r.Content) != 0 {
		t.Errorf("expected no content when body matches status text, got %d", len(r.Content))
	}
}

func TestBuildResponse_HTML(t *testing.T) {
	resp := &cassette.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": {"text/html"}},
		Body:       []byte(`<html><body>hello</body></html>`),
	}

	r, err := buildResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	mt := r.Content["text/html"]
	if mt == nil {
		t.Fatal("expected text/html entry")
	}
	// HTML: no schema, just a placeholder entry
	if mt.Schema != nil {
		t.Error("expected no schema for HTML response")
	}
}

func TestBuildResponse_UnknownStatus(t *testing.T) {
	resp := &cassette.Response{
		StatusCode: 599,
		Headers:    http.Header{},
	}
	r, err := buildResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if r.Description == "" {
		t.Error("expected non-empty description for unknown status code")
	}
}

func TestIsJSONMediaType(t *testing.T) {
	yes := []string{"application/json", "application/vnd.api+json", "application/hal+json"}
	for _, mt := range yes {
		if !isJSONMediaType(mt) {
			t.Errorf("expected %q to be JSON media type", mt)
		}
	}
	no := []string{"text/plain", "text/html", "application/xml", "multipart/form-data"}
	for _, mt := range no {
		if isJSONMediaType(mt) {
			t.Errorf("expected %q NOT to be JSON media type", mt)
		}
	}
}

func TestIsInfraResponseHeader(t *testing.T) {
	infra := []string{"Content-Type", "Cache-Control", "X-Cache", "CF-Ray", "Server"}
	for _, h := range infra {
		if !isInfraResponseHeader(h) {
			t.Errorf("expected %q to be infra header", h)
		}
	}
}
