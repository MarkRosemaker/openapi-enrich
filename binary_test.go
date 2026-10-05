package enrich_test

import (
	"net/http"
	"testing"

	"github.com/MarkRosemaker/openapi"
	enrich "github.com/MarkRosemaker/openapi-enrich"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestEnrich_BinaryBodies(t *testing.T) {
	doc := enrich.NewDocument()

	download := cassette.Interaction{
		Request: cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/exports/7", Headers: http.Header{}},
		Response: cassette.Response{
			StatusCode:  http.StatusOK,
			Headers:     http.Header{"Content-Type": {"application/zip"}},
			BodyOmitted: true,
		},
	}

	upload := cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodPut, URL: "https://api.example.com/files/7",
			Headers:     http.Header{"Content-Type": {"application/pdf"}},
			BodyOmitted: true,
		},
		Response: cassette.Response{StatusCode: http.StatusNoContent, Headers: http.Header{}},
	}

	// twice, so the second merges into what the first made
	if err := enrich.Enrich(doc, cassette.Interactions{download, upload, download, upload}); err != nil {
		t.Fatal(err)
	}

	isBinary := func(c openapi.Content, mr openapi.MediaRange) bool {
		mt, ok := c[mr]
		return ok && mt.Schema != nil && mt.Schema.Type == openapi.TypeString && mt.Schema.Format == openapi.FormatBinary
	}

	var gotDownload, gotUpload bool

	for _, pi := range doc.Paths {
		if pi.Get != nil {
			if r, ok := pi.Get.Responses["200"]; ok {
				gotDownload = isBinary(r.Value.Content, "application/zip")
			}
		}

		if pi.Put != nil && pi.Put.RequestBody != nil {
			gotUpload = isBinary(pi.Put.RequestBody.Value.Content, "application/pdf")
		}
	}

	if !gotDownload {
		t.Error("the zip response is not documented as a string of bytes")
	}

	if !gotUpload {
		t.Error("the PDF upload is not documented as a string of bytes")
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}
