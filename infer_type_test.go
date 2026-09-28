package enrich

import (
	"encoding/json/jsontext"
	"testing"

	"github.com/MarkRosemaker/openapi"
)

func values(vs ...string) []jsontext.Value {
	out := make([]jsontext.Value, len(vs))
	for i, v := range vs {
		out[i] = jsontext.Value(v)
	}

	return out
}

func TestInferType(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		schema   openapi.Schema
		want     openapi.DataType
		nullable bool
	}{
		{"no enum or const", openapi.Schema{}, "", false},
		{"string enum", openapi.Schema{Enum: values(`"a"`, `"b"`)}, openapi.TypeString, false},
		{"integer const", openapi.Schema{Const: jsontext.Value(`401`)}, openapi.TypeInteger, false},
		{"whole float is integer", openapi.Schema{Enum: values(`1.0`, `2`)}, openapi.TypeInteger, false},
		{"fraction makes number", openapi.Schema{Enum: values(`1`, `1.5`)}, openapi.TypeNumber, false},
		{"number then integer", openapi.Schema{Enum: values(`1.5`, `1`)}, openapi.TypeNumber, false},
		{"boolean", openapi.Schema{Enum: values(`true`, `false`)}, openapi.TypeBoolean, false},
		{"array", openapi.Schema{Const: jsontext.Value(`[1]`)}, openapi.TypeArray, false},
		{"object", openapi.Schema{Enum: values(`{"a":1}`)}, openapi.TypeObject, false},
		{"null makes nullable", openapi.Schema{Enum: values(`null`, `"a"`)}, openapi.TypeString, true},
		{"only null", openapi.Schema{Const: jsontext.Value(`null`)}, openapi.TypeNull, false},
		{"enum and const", openapi.Schema{Enum: values(`"a"`), Const: jsontext.Value(`"a"`)}, openapi.TypeString, false},
		{"mixed kinds", openapi.Schema{Enum: values(`"a"`, `1`)}, "", false},
		{"enum and const of different kinds", openapi.Schema{Enum: values(`"a"`), Const: jsontext.Value(`1`)}, "", false},
		{"type is kept", openapi.Schema{Type: openapi.TypeNumber, Enum: values(`1`)}, openapi.TypeNumber, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := tc.schema
			inferType(&s)

			if s.Type != tc.want || s.Nullable != tc.nullable {
				t.Fatalf("got %q (nullable %t), want %q (nullable %t)", s.Type, s.Nullable, tc.want, tc.nullable)
			}
		})
	}
}

func TestInferTypes(t *testing.T) {
	t.Parallel()

	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/a": {
      "parameters": [{"name": "p", "in": "query", "schema": {"enum": ["x"]}}],
      "post": {
        "parameters": [{"name": "q", "in": "query", "content": {"application/json": {"schema": {"const": 1}}}}],
        "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Body"}}}},
        "responses": {
          "200": {
            "description": "ok",
            "headers": {"H": {"schema": {"enum": [true]}}},
            "content": {"application/json": {
              "schema": {"type": "array", "prefixItems": [{"const": "x"}], "items": {"const": "y"}},
              "encoding": {"a": {"headers": {"E": {"schema": {"const": "e"}}}}}
            }}
          }
        },
        "callbacks": {"cb": {"/cb": {"$ref": "#/components/pathItems/Item"}}}
      }
    }
  },
  "webhooks": {"hook": {"$ref": "#/components/pathItems/Item"}},
  "components": {
    "schemas": {
      "Body": {
        "type": "object",
        "properties": {"self": {"$ref": "#/components/schemas/Body"}, "k": {"enum": ["a"]}},
        "additionalProperties": {"type": "number", "anyOf": [{"const": 1.5}], "oneOf": [{"const": 2}], "allOf": [{"const": 3}], "not": {"const": "n"}}
      }
    },
    "responses": {"R": {"description": "r", "content": {"application/json": {"schema": {"const": "r"}}}}},
    "parameters": {"P": {"name": "p", "in": "header", "schema": {"const": "p"}}},
    "requestBodies": {"B": {"content": {"application/json": {"schema": {"const": "b"}}}}},
    "headers": {"H": {"schema": {"const": "h"}}},
    "callbacks": {"C": {"/c": {"get": {"responses": {"200": {"description": "c", "content": {"application/json": {"schema": {"const": "c"}}}}}}}}},
    "pathItems": {"Item": {"get": {"responses": {"200": {"description": "i", "content": {"application/json": {"schema": {"const": "i"}}}}}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	inferTypes(doc)

	visited, untyped := 0, 0
	walkSchemas(doc, func(s *openapi.Schema) {
		visited++

		if s.Type == "" {
			untyped++
		}
	})

	if untyped != 0 {
		t.Fatalf("%d schemas are left without a type", untyped)
	}

	if visited != 20 {
		t.Fatalf("visited %d schemas, want 20", visited)
	}
}
