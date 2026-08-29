package enrich_test

import (
	"encoding/json/jsontext"
	"testing"

	"github.com/MarkRosemaker/openapi"
	enrich "github.com/MarkRosemaker/openapi-enrich"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestEnum(t *testing.T) {
	doc, err := openapi.LoadFromFile("testdata/retrodiffusion/golden.json")
	if err != nil {
		t.Fatal(err)
	}

	promptStyle := doc.Paths["/v1/styles/selector"].Get.Responses["200"].Value.Content["application/json"].Schema.Value.Properties["styles"].Value.Items.Value.Properties["prompt_style"].Value
	if len(promptStyle.Enum) > 0 {
		t.Fatalf("initially, prompt style has no enum")
	}

	// set an enum
	promptStyle.Enum = []jsontext.Value{promptStyle.Example}

	ias, err := cassette.InteractionsReadFile("testdata/retrodiffusion/interactions.json")
	if err != nil {
		t.Fatal(err)
	}

	if err := enrich.Enrich(doc, ias); err != nil {
		t.Fatal(err)
	}

	if len(promptStyle.Enum) <= 1 {
		t.Fatalf("wanted more than one enum, got: %d", len(promptStyle.Enum))
	}
}
