package migrate_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MarkRosemaker/openapi-enrich/migrate"
)

// writeCassette writes content to a temp file named <base>.yaml and returns
// the full path (with extension) for use with FromGoVCRFile.
func writeCassette(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "cassette.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// fixtureMulti is a two-interaction go-vcr v2 cassette: a GET followed by a POST.
const fixtureMulti = `---
version: 2
interactions:
- id: 0
  request:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    content_length: 0
    transfer_encoding: []
    trailer: {}
    host: api.example.com
    remote_addr: ""
    request_uri: ""
    body: ""
    form: {}
    headers:
      Accept-Encoding:
      - gzip
    method: GET
    url: https://api.example.com/users
  response:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    transfer_encoding: []
    trailer: {}
    content_length: 26
    uncompressed: false
    body: '[{"id":1,"name":"Alice"}]'
    headers:
      Content-Type:
      - application/json
    status: "200 OK"
    code: 200
    duration: 1ms
- id: 1
  request:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    content_length: 14
    transfer_encoding: []
    trailer: {}
    host: api.example.com
    remote_addr: ""
    request_uri: ""
    body: '{"name":"Bob"}'
    form: {}
    headers:
      Content-Type:
      - application/json
    method: POST
    url: https://api.example.com/users
  response:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    transfer_encoding: []
    trailer: {}
    content_length: 21
    uncompressed: false
    body: '{"id":2,"name":"Bob"}'
    headers:
      Content-Type:
      - application/json
    status: "201 Created"
    code: 201
    duration: 2ms
`

// fixtureSingle is a minimal single-interaction go-vcr v2 cassette with a plain-text body.
const fixtureSingle = `---
version: 2
interactions:
- id: 0
  request:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    content_length: 0
    transfer_encoding: []
    trailer: {}
    host: api.example.com
    remote_addr: ""
    request_uri: ""
    body: ""
    form: {}
    headers:
      Accept-Encoding:
      - gzip
    url: https://api.example.com/ping
    method: GET
  response:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    transfer_encoding: []
    trailer: {}
    content_length: 4
    uncompressed: false
    body: pong
    headers:
      Content-Type:
      - text/plain
    status: "200 OK"
    code: 200
    duration: 500µs
`

func TestFromGoVCRFile_MultipleInteractions(t *testing.T) {
	ias, err := migrate.FromGoVCRFile(writeCassette(t, fixtureMulti))
	if err != nil {
		t.Fatalf("FromGoVCRFile: %v", err)
	}

	if len(ias) != 2 {
		t.Fatalf("got %d interactions, want 2", len(ias))
	}

	// First interaction — GET
	ia0 := ias[0]
	if ia0.Request.Method != http.MethodGet {
		t.Errorf("method: got %q, want GET", ia0.Request.Method)
	}
	if ia0.Request.URL != "https://api.example.com/users" {
		t.Errorf("url: got %q", ia0.Request.URL)
	}
	if ia0.Request.Body != nil {
		t.Errorf("body: expected nil for empty body, got %q", ia0.Request.Body)
	}
	if ia0.Response.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", ia0.Response.StatusCode)
	}
	if string(ia0.Response.Body) != `[{"id":1,"name":"Alice"}]` {
		t.Errorf("response body: got %q", ia0.Response.Body)
	}
	if ia0.Response.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("content-type: got %q", ia0.Response.Headers.Get("Content-Type"))
	}

	// Second interaction — POST with request body
	ia1 := ias[1]
	if ia1.Request.Method != http.MethodPost {
		t.Errorf("method: got %q, want POST", ia1.Request.Method)
	}
	if string(ia1.Request.Body) != `{"name":"Bob"}` {
		t.Errorf("request body: got %q", ia1.Request.Body)
	}
	if ia1.Response.StatusCode != 201 {
		t.Errorf("status: got %d, want 201", ia1.Response.StatusCode)
	}
}

func TestFromGoVCRFile_PlainTextBody(t *testing.T) {
	ias, err := migrate.FromGoVCRFile(writeCassette(t, fixtureSingle))
	if err != nil {
		t.Fatalf("FromGoVCRFile: %v", err)
	}

	if len(ias) != 1 {
		t.Fatalf("got %d interactions, want 1", len(ias))
	}

	ia := ias[0]
	if ia.Request.Method != http.MethodGet {
		t.Errorf("method: got %q, want GET", ia.Request.Method)
	}
	if ia.Request.URL != "https://api.example.com/ping" {
		t.Errorf("url: got %q", ia.Request.URL)
	}
	if ia.Request.Body != nil {
		t.Errorf("body: expected nil for empty body")
	}
	if ia.Response.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", ia.Response.StatusCode)
	}
	// Plain-text body should be preserved as raw bytes.
	if string(ia.Response.Body) != "pong" {
		t.Errorf("response body: got %q, want %q", ia.Response.Body, "pong")
	}
}

func TestFromGoVCRFile_EmptyBodies(t *testing.T) {
	const input = `---
version: 2
interactions:
- id: 0
  request:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    content_length: 0
    transfer_encoding: []
    trailer: {}
    host: api.example.com
    remote_addr: ""
    request_uri: ""
    body: ""
    form: {}
    headers: {}
    method: DELETE
    url: https://api.example.com/item
  response:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    transfer_encoding: []
    trailer: {}
    content_length: 0
    uncompressed: false
    body: ""
    headers: {}
    status: "204 No Content"
    code: 204
    duration: 1ms
`
	ias, err := migrate.FromGoVCRFile(writeCassette(t, input))
	if err != nil {
		t.Fatalf("FromGoVCRFile: %v", err)
	}

	if len(ias) != 1 {
		t.Fatalf("got %d interactions, want 1", len(ias))
	}

	ia := ias[0]
	if ia.Request.Headers != nil {
		t.Errorf("request headers: expected nil for empty map, got %v", ia.Request.Headers)
	}
	if ia.Response.Headers != nil {
		t.Errorf("response headers: expected nil for empty map, got %v", ia.Response.Headers)
	}
	if ia.Request.Body != nil {
		t.Errorf("request body: expected nil for empty string")
	}
	if ia.Response.Body != nil {
		t.Errorf("response body: expected nil for empty string")
	}
	if ia.Response.StatusCode != 204 {
		t.Errorf("status: got %d, want 204", ia.Response.StatusCode)
	}
}

func TestFromGoVCRFile_WithExtension(t *testing.T) {
	// Passing the full path including ".yaml" should work identically.
	ias, err := migrate.FromGoVCRFile(writeCassette(t, fixtureMulti))
	if err != nil {
		t.Fatalf("FromGoVCRFile (with .yaml): %v", err)
	}

	if len(ias) != 2 {
		t.Fatalf("got %d interactions, want 2", len(ias))
	}
}

func TestFromGoVCRFile_WithoutExtension(t *testing.T) {
	// go-vcr's Load expects the name without ".yaml"; both forms should work.
	path := writeCassette(t, fixtureMulti)
	nameWithout := strings.TrimSuffix(path, ".yaml")

	ias, err := migrate.FromGoVCRFile(nameWithout)
	if err != nil {
		t.Fatalf("FromGoVCRFile (without .yaml): %v", err)
	}

	if len(ias) != 2 {
		t.Fatalf("got %d interactions, want 2", len(ias))
	}
}

func TestFromGoVCRFile_NotFound(t *testing.T) {
	_, err := migrate.FromGoVCRFile(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
