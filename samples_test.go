package enrich

import (
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"

	"github.com/MarkRosemaker/openapi"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func blocksInteraction(body string) cassette.Interaction {
	return cassette.Interaction{
		Request: cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/blocks"},
		Response: cassette.Response{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": {"application/json"}},
			Body:       []byte(body),
		},
	}
}

func TestEnrich_ArrayOfVariants(t *testing.T) {
	// each element of a list of mixed variants reaches its own; merged into one, they would reach neither
	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "servers": [{"url": "https://api.example.com"}],
  "paths": {"/blocks": {"get": {"operationId": "listBlocks", "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {
    "type": "object", "properties": {"results": {"type": "array", "items": {"$ref": "#/components/schemas/Block"}}}}}}}}}}},
  "components": {"schemas": {
    "Block": {"oneOf": [{"$ref": "#/components/schemas/Paragraph"}, {"$ref": "#/components/schemas/Heading"}]},
    "Paragraph": {"type": "object", "properties": {"type": {"type": "string", "const": "paragraph"}, "paragraph": {"type": "string"}}},
    "Heading": {"type": "object", "properties": {"type": {"type": "string", "const": "heading"}, "heading": {"type": "string"}}}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}

	if err := Enrich(doc, cassette.Interactions{
		blocksInteraction(`{"results":[{"type":"paragraph","paragraph":"a"},{"type":"heading","heading":"b"}]}`),
	}); err != nil {
		t.Fatal(err)
	}

	for name, own := range map[string]string{"Paragraph": "paragraph", "Heading": "heading"} {
		props := doc.Components.Schemas[name].Properties
		if len(props) != 2 || props[own] == nil {
			t.Errorf("%s: got properties %v, want type and %s only", name, props, own)
		}
	}
}

func TestEnrich_ArrayOfObjectsWithoutSpec(t *testing.T) {
	// with no specification to route them, the elements merge into one item, as they always did
	doc := NewDocument()
	if err := Enrich(doc, cassette.Interactions{
		blocksInteraction(`{"results":[{"type":"paragraph","paragraph":"a"},{"type":"heading","heading":"b"}]}`),
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "x-samples") || strings.Contains(string(data), "anyOf") {
		t.Errorf("samples left in %s", data)
	}

	items := doc.Paths["/blocks"].Get.Responses["200"].Value.Content["application/json"].Schema.Properties["results"].Items
	if items.Type != openapi.TypeObject || items.Properties["paragraph"] == nil || items.Properties["heading"] == nil {
		t.Errorf("got items %+v, want one object of both", items)
	}
}

func TestEnrich_DateOrDateTime(t *testing.T) {
	// Notion's date start is a date or a date-time, as the property has a time or not
	doc := NewDocument()
	if err := Enrich(doc, cassette.Interactions{
		blocksInteraction(`{"start":"2026-10-05"}`),
		blocksInteraction(`{"start":"2026-10-05T10:00:00.000+00:00"}`),
	}); err != nil {
		t.Fatal(err)
	}

	start := doc.Paths["/blocks"].Get.Responses["200"].Value.Content["application/json"].Schema.Properties["start"]
	if len(start.OneOf) != 2 || start.OneOf[0].Format != openapi.FormatDate || start.OneOf[1].Format != openapi.FormatDateTime {
		t.Errorf("got %+v, want a date or a date-time", start)
	}
}
