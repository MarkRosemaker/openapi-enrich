package enrich_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/MarkRosemaker/cassette"
	"github.com/MarkRosemaker/openapi"
	enrich "github.com/MarkRosemaker/openapi-enrich"
)

// TestEnrich_Golden enriches testdata/openapi.json with testdata/interactions.json, three times over, and must get
// testdata/golden.json each time: enriching again with what was already recorded changes nothing. All three files are
// edited by hand; the interactions are grouped by what each shows, and the document says what they meet in it.
func TestEnrich_Golden(t *testing.T) {
	t.Parallel()

	doc, err := openapi.LoadFromFile("testdata/openapi.json")
	if err != nil {
		t.Fatal(err)
	}

	interactions, err := cassette.InteractionsReadFile("testdata/interactions.json")
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}

	for run := range 3 {
		if err := enrich.Enrich(doc, interactions); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}

		if err := doc.Validate(); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}

		got, err := doc.ToJSON()
		if err != nil {
			t.Fatal(err)
		}

		got = append(got, '\n')

		if line, ok := firstDifference(got, want); !ok {
			t.Fatalf("run %d: golden.json %s", run+1, line)
		}
	}
}

// firstDifference describes the first line in which got and want differ, if any.
func firstDifference(got, want []byte) (string, bool) {
	if bytes.Equal(got, want) {
		return "", true
	}

	gotLines, wantLines := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
	for i := range min(len(gotLines), len(wantLines)) {
		if !bytes.Equal(gotLines[i], wantLines[i]) {
			return fmt.Sprintf("line %d: got %s, want %s", i+1, bytes.TrimSpace(gotLines[i]), bytes.TrimSpace(wantLines[i])), false
		}
	}

	return fmt.Sprintf("has %d lines, got %d", len(wantLines), len(gotLines)), false
}

func TestEnrich_Empty(t *testing.T) {
	t.Parallel()

	doc := enrich.NewDocument()
	if err := enrich.Enrich(doc, nil); err != nil {
		t.Fatalf("Enrich(nil) error: %v", err)
	}

	if err := enrich.Enrich(doc, cassette.Interactions{}); err != nil {
		t.Fatalf("Enrich(empty) error: %v", err)
	}
}

// TestEnrich_PathTemplateSegmentMatch_Deterministic guards against a request matching a shorter path template whose
// trailing {param} absorbs extra segments (/things/{thingId} swallowing "a/start") instead of the template of its
// length (/things/{thingId}/start). That choice once fell out of map iteration order and failed only some of the time,
// which one run, as the golden test makes, could miss; so this runs many times, fresh each time.
func TestEnrich_PathTemplateSegmentMatch_Deterministic(t *testing.T) {
	t.Parallel()

	const spec = `{
  "openapi": "3.1.0",
  "info": {"title": "things", "version": "1"},
  "servers": [{"url": "http://localhost:8083/api"}],
  "paths": {
    "/things/{thingId}": {
      "parameters": [{"name": "thingId", "in": "path", "required": true, "schema": {"type": "string"}}],
      "get": {"operationId": "GetThing", "responses": {"200": {"description": "OK"}}}
    },
    "/things/{thingId}/start": {
      "parameters": [{"name": "thingId", "in": "path", "required": true, "schema": {"type": "string"}}],
      "post": {"operationId": "StartThing", "responses": {"200": {"description": "OK"}}}
    }
  }
}`

	for i := range 200 {
		doc, err := openapi.LoadFromDataJSON([]byte(spec))
		if err != nil {
			t.Fatal(err)
		}

		if err := enrich.Enrich(doc, cassette.Interactions{{
			Request:  cassette.Request{Method: http.MethodPost, URL: url.URL{Scheme: "http", Host: "localhost:8083", Path: "/api/things/a/start"}},
			Response: cassette.Response{StatusCode: http.StatusOK},
		}}); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}

		if len(doc.Paths) != 2 || doc.Paths["/things/{thingId}"].Post != nil {
			t.Fatalf("run %d: the call reached GetThing's template", i)
		}
	}
}
