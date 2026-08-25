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
		opWithSecurity(openapi.SecurityRequirement{"bearerAuth": {}}),
		opWithSecurity(openapi.SecurityRequirement{"bearerAuth": {}}),
	)

	hoistSecurity(doc)

	// expect bearerAuth hoisted to doc level
	if got, want := len(doc.Security), 1; got != want {
		t.Errorf("len(doc.Security)=%d, want=%d", got, want)
	} else if got, want := doc.Security[0], (openapi.SecurityRequirement{"bearerAuth": {}}); !got.Equals(want) {
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
		opWithSecurity(openapi.SecurityRequirement{"bearerAuth": {}}),
		opWithSecurity(), // no security
	)

	hoistSecurity(doc)

	if doc.Security != nil {
		t.Errorf("expected no hoisting when not all ops have the requirement, got %v", doc.Security)
	}
}

func TestHoistSecurity_MultipleReqs_OnlyCommonHoisted(t *testing.T) {
	// bearerAuth is on both ops; apiKey is only on one.
	// Only bearerAuth should be hoisted.
	bearer := openapi.SecurityRequirement{"bearerAuth": {}}
	apiKey := openapi.SecurityRequirement{"apiKey": {}}

	doc := docWithOps(
		opWithSecurity(bearer, apiKey),
		opWithSecurity(bearer),
	)

	hoistSecurity(doc)

	if !doc.Security.Contains(bearer) {
		t.Error("expected bearerAuth hoisted to doc level")
	}
	if doc.Security.Contains(apiKey) {
		t.Error("apiKey should NOT be hoisted (not on all ops)")
	}
	// The op that had apiKey should still have it.
	ops := allOperations(doc)
	found := false
	for _, op := range ops {
		if op.Security.Contains(apiKey) {
			found = true
		}
	}
	if !found {
		t.Error("apiKey should remain on the op that originally had it")
	}
}

func TestHoistSecurity_NoOps(t *testing.T) {
	doc := NewDocument()
	hoistSecurity(doc) // must not panic
	if doc.Security != nil {
		t.Error("expected no security on empty doc")
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

func TestEnrich_SecurityHoisted(t *testing.T) {
	// Integration: Bearer auth observed on every request should be hoisted.
	doc := NewDocument()
	interactions := cassette.Interactions{
		{
			Request: cassette.Request{
				Method:  http.MethodGet,
				URL:     "https://api.example.com/users",
				Headers: http.Header{"Authorization": {"Bearer tok1"}},
			},
			Response: cassette.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": {"application/json"}},
				Body:       []byte(`[]`),
			},
		},
		{
			Request: cassette.Request{
				Method:  http.MethodGet,
				URL:     "https://api.example.com/posts",
				Headers: http.Header{"Authorization": {"Bearer tok1"}},
			},
			Response: cassette.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": {"application/json"}},
				Body:       []byte(`[]`),
			},
		},
	}

	if err := Enrich(doc, interactions); err != nil {
		t.Fatalf("Enrich error: %v", err)
	}

	if !doc.Security.Contains(openapi.SecurityRequirement{"bearerAuth": {}}) {
		t.Error("expected bearerAuth at doc level")
	}
	for _, pi := range doc.Paths {
		for _, op := range pi.Operations {
			if len(op.Security) != 0 {
				t.Errorf("expected op-level security to be empty after hoisting, got %v", op.Security)
			}
		}
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
