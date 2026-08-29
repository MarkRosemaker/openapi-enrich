package enrich_test

import (
	"bytes"
	"embed"
	"encoding/json/jsontext"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/MarkRosemaker/openapi"
	enrich "github.com/MarkRosemaker/openapi-enrich"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

//go:embed testdata
var testdata embed.FS

func TestEnrich_TestData(t *testing.T) {
	entries, err := testdata.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range entries {
		t.Run(tc.Name(), func(t *testing.T) {
			path := filepath.Join("testdata", tc.Name(), "interactions.json")
			f, err := testdata.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close() //nolint:errcheck

			interactions, err := cassette.InteractionsUnmarshalRead(f)
			if err != nil {
				t.Fatal(err)
			}

			wantDoc, err := testdata.ReadFile(filepath.Join("testdata", tc.Name(), "golden.json"))
			if err != nil {
				t.Fatal(err)
			}

			doc := enrich.NewDocument()

			if data, err := testdata.ReadFile(filepath.Join("testdata", tc.Name(), "openapi.json")); err == nil {
				doc, err = openapi.LoadFromDataJSON(data)
				if err != nil {
					t.Fatalf("loading initial spec: %v", err)
				}
			}

			for it := range 3 {
				t.Run(fmt.Sprintf("iteration %d", it+1), func(t *testing.T) {
					if err := enrich.Enrich(doc, interactions); err != nil {
						t.Fatal(err)
					}

					if err := doc.Validate(); err != nil {
						t.Fatal(err)
					}

					// Sort responses and components (but not paths to keep the order)
					for _, path := range doc.Paths {
						for _, op := range path.Operations {
							op.Responses.Sort()
						}
					}
					doc.Components.SortMaps()

					gotDoc, err := doc.ToJSON()
					if err != nil {
						t.Fatal(err)
					}

					compareBytes(t, wantDoc, gotDoc)
				})
			}
		})
	}
}

// compareBytes prints a compact diff of two byte slices
func compareBytes(t *testing.T, expected, actual []byte) {
	t.Helper()

	if bytes.Equal(expected, actual) {
		return
	}

	// Find first difference
	i := 0
	for i < len(expected) && i < len(actual) && expected[i] == actual[i] {
		i++
	}

	t.Errorf("\n┌─ Diff at offset %d\n│ Expected: %q\n│ Actual:   %q\n└─ %s",
		i, expected[i:min(len(expected), i+20)], actual[i:min(len(actual), i+20)],
		func() string {
			if len(expected) != len(actual) {
				return fmt.Sprintf("length %d vs %d", len(expected), len(actual))
			}
			return fmt.Sprintf("0x%02x vs 0x%02x", expected[i], actual[i])
		}())
}

func TestEnrich_Basic(t *testing.T) {
	doc := enrich.NewDocument()
	interactions := cassette.Interactions{
		{
			Request: cassette.Request{
				Method:  http.MethodGet,
				URL:     "https://api.example.com/users",
				Headers: http.Header{},
			},
			Response: cassette.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": {"application/json"}},
				Body:       []byte(`[{"id":1,"name":"Alice"}]`),
			},
		},
		{
			Request: cassette.Request{
				Method: http.MethodPost,
				URL:    "https://api.example.com/users",
				Headers: http.Header{
					"Content-Type":  {"application/json"},
					"Authorization": {"Bearer token123"},
				},
				Body: []byte(`{"name":"Bob"}`),
			},
			Response: cassette.Response{
				StatusCode: 201,
				Headers:    http.Header{"Content-Type": {"application/json"}},
				Body:       []byte(`{"id":2,"name":"Bob"}`),
			},
		},
		{
			Request: cassette.Request{
				Method:  http.MethodGet,
				URL:     "https://api.example.com/users/42",
				Headers: http.Header{},
			},
			Response: cassette.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": {"application/json"}},
				Body:       []byte(`{"id":42,"name":"Carol"}`),
			},
		},
	}

	if err := enrich.Enrich(doc, interactions); err != nil {
		t.Fatalf("Enrich error: %v", err)
	}

	if len(doc.Paths) == 0 {
		t.Error("expected paths to be populated")
	}

	// /users should exist
	users := doc.Paths["/users"]
	if users == nil {
		t.Fatal("expected /users path")
	}
	if users.Get == nil {
		t.Error("expected GET /users")
	}
	if users.Post == nil {
		t.Error("expected POST /users")
	}

	// /users/{id} (or similar) should exist
	found := false
	for path := range doc.Paths {
		if path != "/users" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a parametric path for /users/{id}")
	}
}

func TestEnrich_Empty(t *testing.T) {
	doc := enrich.NewDocument()
	if err := enrich.Enrich(doc, nil); err != nil {
		t.Fatalf("Enrich(nil) error: %v", err)
	}
	if err := enrich.Enrich(doc, cassette.Interactions{}); err != nil {
		t.Fatalf("Enrich(empty) error: %v", err)
	}
}

func TestEnrich_InvalidURL(t *testing.T) {
	doc := enrich.NewDocument()
	interactions := cassette.Interactions{
		{
			Request: cassette.Request{
				Method:  http.MethodGet,
				URL:     "://bad-url",
				Headers: http.Header{},
			},
			Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
		},
	}

	err := enrich.Enrich(doc, interactions)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestEnrich_ExistingParameter(t *testing.T) {
	doc := enrich.NewDocument()

	doc.Servers = append(doc.Servers, openapi.Server{
		URL: "https://api.notion.com/v1",
	})

	header := &openapi.Parameter{
		Name:     "Notion-Version",
		In:       openapi.ParameterLocationHeader,
		Required: true,
		Schema: &openapi.SchemaRef{Value: &openapi.Schema{
			Type:    openapi.TypeString,
			Example: jsontext.Value(`"2026-03-11"`),
		}},
	}
	doc.Components.Parameters.Set("NotionVersionHeader", &openapi.ParameterRef{
		Value: header,
	})
	doc.Paths.Set("/pages/{id}", &openapi.PathItem{
		Parameters: openapi.ParameterList{
			{
				Ref:   &openapi.Reference{Identifier: "#/components/parameters/NotionVersionHeader"},
				Value: header,
			},
			{Value: &openapi.Parameter{
				Name:     "id",
				In:       openapi.ParameterLocationPath,
				Required: true,
				Schema: &openapi.SchemaRef{Value: &openapi.Schema{
					Type:   openapi.TypeString,
					Format: openapi.FormatUUID,
				}},
			}},
		},
		Get: &openapi.Operation{
			OperationID: "GetPage",
			Responses: openapi.OperationResponses{
				"200": &openapi.ResponseRef{
					Value: &openapi.Response{
						Description: "some description",
					},
				},
			},
		},
	})

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}

	wantDoc, err := doc.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	interactions := cassette.Interactions{
		{
			Request: cassette.Request{
				Method: http.MethodGet,
				URL:    "https://api.notion.com/v1/pages/96245c8f-1784-44a4-82ad-1941127c3ec3",
				Headers: http.Header{
					"Notion-Version": []string{"2026-03-11"},
				},
			},
			Response: cassette.Response{StatusCode: http.StatusOK},
		},
	}

	if err := enrich.Enrich(doc, interactions); err != nil {
		t.Fatalf("Enrich error: %v", err)
	}

	gotDoc, err := doc.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	compareBytes(t, wantDoc, gotDoc)
}
