package enrich

import (
	"net/http"
	"testing"

	"github.com/MarkRosemaker/openapi"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestHoistSecurity_AllOpsHaveSameReq(t *testing.T) {
	// When every operation has the same security requirement it should be
	// hoisted to document level and removed from the operations.
	doc := docWithOps(
		opWithSecurity(openapi.SecurityRequirement{schemeNameBearer: {}}),
		opWithSecurity(openapi.SecurityRequirement{schemeNameBearer: {}}),
	)

	hoistSecurity(doc)

	// expect bearerAuth hoisted to doc level
	if got, want := len(doc.Security), 1; got != want {
		t.Errorf("len(doc.Security)=%d, want=%d", got, want)
	} else if got, want := doc.Security[0], (openapi.SecurityRequirement{schemeNameBearer: {}}); !got.Equals(want) {
		t.Errorf("doc.Security[0]=%v, want=%v", got, want)
	}

	for _, pi := range doc.Paths {
		for _, op := range pi.Operations {
			if op.Security != nil {
				t.Error("expected op-level security to be cleared after hoisting")
			}
		}
	}
}

func TestHoistSecurity_NotAllOpsHaveReq(t *testing.T) {
	// If not every operation carries a requirement it must NOT be hoisted.
	doc := docWithOps(
		opWithSecurity(openapi.SecurityRequirement{schemeNameBearer: {}}),
		opWithSecurity(), // no security
	)

	hoistSecurity(doc)

	if doc.Security != nil {
		t.Errorf("expected no hoisting when not all ops have the requirement, got %v", doc.Security)
	}
}

func TestHoistSecurity_PartOfAList(t *testing.T) {
	// The first op accepts either scheme. Hoisting bearerAuth alone would leave it requiring apiKey.
	bearer := openapi.SecurityRequirement{schemeNameBearer: {}}
	apiKey := openapi.SecurityRequirement{"apiKey": {}}

	doc := docWithOps(
		opWithSecurity(bearer, apiKey),
		opWithSecurity(bearer),
	)

	hoistSecurity(doc)

	if doc.Security != nil {
		t.Fatalf("expected no hoisting, got %v", doc.Security)
	}

	if ops := allOperations(doc); !sameRequirements(ops[0].Security, openapi.SecurityRequirements{bearer, apiKey}) &&
		!sameRequirements(ops[1].Security, openapi.SecurityRequirements{bearer, apiKey}) {
		t.Errorf("an op should still accept either scheme, got %v and %v", ops[0].Security, ops[1].Security)
	}
}

func TestHoistSecurity_NoOps(t *testing.T) {
	doc := NewDocument()
	hoistSecurity(doc) // must not panic

	if doc.Security != nil {
		t.Error("expected no security on empty doc")
	}
}

func TestHoistSecurity_NoneNeeded(t *testing.T) {
	// The document defines a scheme, so it must say that no operation needs it: codegen would send it otherwise.
	doc := docWithOps(opWithSecurity(), opWithSecurity())
	doc.Components.SecuritySchemes = openapi.SecuritySchemes{}
	doc.Components.SecuritySchemes.Set(schemeNameBearer, &openapi.SecuritySchemeRef{Value: &openapi.SecurityScheme{
		Type: openapi.SecuritySchemeTypeHTTP, Scheme: openapi.SecuritySchemeBearer,
	}})

	hoistSecurity(doc)

	if doc.Security == nil || len(doc.Security) != 0 {
		t.Errorf("document security is %#v, want none stated", doc.Security)
	}

	for _, op := range allOperations(doc) {
		if op.Security != nil {
			t.Errorf("op security is %#v, want it hoisted", op.Security)
		}
	}
}

func TestHoistSecurity_NoSecurity(t *testing.T) {
	doc := docWithOps(
		opWithSecurity(), // explicit empty
		opWithSecurity(),
	)
	hoistSecurity(doc)

	if doc.Security != nil {
		t.Error("expected no hoisting when ops have no security")
	}
}

func TestEnrich_Unauthenticated(t *testing.T) {
	bearer := openapi.SecurityRequirement{schemeNameBearer: {}}

	call := func(path string, auth bool) *cassette.Interaction {
		h := http.Header{}
		if auth {
			h.Set("Authorization", "Bearer tok1")
		}

		return &cassette.Interaction{
			Request: cassette.Request{Method: http.MethodGet, URL: "https://api.example.com" + path, Headers: h},
			Response: cassette.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": {"application/json"}},
				Body:       []byte(`[]`),
			},
		}
	}

	for _, tc := range []struct {
		name  string
		calls cassette.Interactions
		doc   openapi.SecurityRequirements
		ops   map[string]openapi.SecurityRequirements
	}{
		{
			name:  "one operation without credentials",
			calls: cassette.Interactions{call("/users", true), call("/health", false)},
			ops: map[string]openapi.SecurityRequirements{
				"/users":  {bearer},
				"/health": {},
			},
		},
		{
			name:  "optional credentials, either order",
			calls: cassette.Interactions{call("/users", true), call("/users", false), call("/posts", false), call("/posts", true)},
			doc:   openapi.SecurityRequirements{anonymous, bearer},
			ops:   map[string]openapi.SecurityRequirements{"/users": nil, "/posts": nil},
		},
		{
			name:  "no operation needs credentials",
			calls: cassette.Interactions{call("/users", false), call("/posts", false)},
			ops:   map[string]openapi.SecurityRequirements{"/users": nil, "/posts": nil},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := NewDocument()
			if err := Enrich(doc, tc.calls); err != nil {
				t.Fatal(err)
			}

			if !sameRequirements(doc.Security, tc.doc) || (doc.Security == nil) != (tc.doc == nil) {
				t.Errorf("document security is %v, want %v", doc.Security, tc.doc)
			}

			for path, want := range tc.ops {
				got := doc.Paths[openapi.Path(path)].Get.Security
				if !sameRequirements(got, want) || (got == nil) != (want == nil) {
					t.Errorf("%s security is %#v, want %#v", path, got, want)
				}
			}
		})
	}
}

func TestEnrich_UnauthenticatedAfterHoisting(t *testing.T) {
	// A document enriched before holds bearerAuth at its level; a new call without credentials overrides it.
	bearer := openapi.SecurityRequirement{schemeNameBearer: {}}
	doc := docWithOps(&openapi.Operation{})
	doc.Security = openapi.SecurityRequirements{bearer}
	doc.Components.SecuritySchemes = openapi.SecuritySchemes{}
	doc.Components.SecuritySchemes.Set(schemeNameBearer, &openapi.SecuritySchemeRef{Value: &openapi.SecurityScheme{
		Type: openapi.SecuritySchemeTypeHTTP, Scheme: openapi.SecuritySchemeBearer,
	}})

	if err := Enrich(doc, cassette.Interactions{{
		Request:  cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/health", Headers: http.Header{}},
		Response: cassette.Response{StatusCode: http.StatusNoContent},
	}}); err != nil {
		t.Fatal(err)
	}

	if got := doc.Paths["/health"].Get.Security; got == nil || len(got) != 0 {
		t.Errorf("security is %#v, want none", got)
	}

	if !doc.Security.Contains(bearer) {
		t.Errorf("document security is %v, want bearerAuth kept", doc.Security)
	}

	// the other operation held bearerAuth from before, so a call without credentials makes them optional
	if err := Enrich(doc, cassette.Interactions{{
		Request:  cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/a", Headers: http.Header{}},
		Response: cassette.Response{StatusCode: http.StatusNoContent},
	}}); err != nil {
		t.Fatal(err)
	}

	if got, want := doc.Paths["/a"].Get.Security, (openapi.SecurityRequirements{anonymous, bearer}); !sameRequirements(got, want) {
		t.Errorf("security is %v, want %v", got, want)
	}
}

// helpers

func docWithOps(ops ...*openapi.Operation) *openapi.Document {
	doc := NewDocument()
	doc.Paths = openapi.Paths{}
	for i, op := range ops {
		pi := &openapi.PathItem{}
		pi.SetOperation(http.MethodGet, op)
		doc.Paths.Set(openapi.Path("/"+string(rune('a'+i))), pi)
	}

	return doc
}

func opWithSecurity(reqs ...openapi.SecurityRequirement) *openapi.Operation {
	op := &openapi.Operation{}
	op.Security = openapi.SecurityRequirements(reqs)
	return op
}

func allOperations(doc *openapi.Document) []*openapi.Operation {
	var ops []*openapi.Operation
	for _, pi := range doc.Paths {
		for _, op := range pi.Operations {
			ops = append(ops, op)
		}
	}

	return ops
}
