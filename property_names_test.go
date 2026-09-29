package enrich

import (
	"encoding/json/jsontext"
	"testing"

	"github.com/MarkRosemaker/openapi"
)

func TestGrowEnums_PropertyNames(t *testing.T) {
	t.Parallel()

	keys := &openapi.Schema{Type: openapi.TypeString, Enum: values(`"north"`)}
	s := &openapi.Schema{
		Type:                 openapi.TypeObject,
		PropertyNames:        keys,
		AdditionalProperties: &openapi.AdditionalProperties{Schema: &openapi.Schema{Type: openapi.TypeString}},
	}

	if err := growEnums(s, []byte(`{"north": "a", "password": "c"}`)); err != nil {
		t.Fatal(err)
	}

	// an observed key joins the enum, even one whose value is redacted
	var got []string
	for _, v := range keys.Enum {
		got = append(got, string(v))
	}

	if len(got) != 2 || got[0] != `"north"` || got[1] != `"password"` {
		t.Fatalf("got %v, want north and password", got)
	}
}

func TestInferTypes_PropertyNames(t *testing.T) {
	t.Parallel()

	keys := &openapi.Schema{Enum: []jsontext.Value{jsontext.Value(`"north"`)}}
	doc := &openapi.Document{Components: openapi.Components{Schemas: openapi.Schemas{
		"Directions": {Type: openapi.TypeObject, PropertyNames: keys},
	}}}

	inferTypes(doc)

	if keys.Type != openapi.TypeString {
		t.Fatalf("got type %q, want string", keys.Type)
	}
}
