package enrich

import (
	"net/http"
	"testing"

	"github.com/MarkRosemaker/openapi"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestAnalyzeInteraction_ServerInit(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/users",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	if len(doc.Servers) == 0 {
		t.Fatal("expected server to be initialized")
	}
}

func TestAnalyzeInteraction_AltHostPathLevelServer(t *testing.T) {
	// A request from a host different from the document server should produce
	// a path item with a path-level servers entry for that host.
	doc := NewDocument()

	// First request establishes the document server.
	if err := analyzeInteraction(doc, &cassette.Interaction{
		Request:  cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/users", Headers: http.Header{}},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}); err != nil {
		t.Fatal(err)
	}

	// Second request hits a different host.
	if err := analyzeInteraction(doc, &cassette.Interaction{
		Request:  cassette.Request{Method: http.MethodGet, URL: "https://other.example.com/tickers", Headers: http.Header{}},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}); err != nil {
		t.Fatal(err)
	}

	if len(doc.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(doc.Paths))
	}

	// Primary-host path: no path-level servers override.
	if pi := doc.Paths["/users"]; len(pi.Servers) != 0 {
		t.Errorf("/users: expected no path-level servers, got %v", pi.Servers)
	}

	// Alt-host path: carries its own servers entry.
	pi := doc.Paths["/tickers"]
	if pi == nil {
		t.Fatal("expected /tickers path")
	}
	if len(pi.Servers) != 1 || pi.Servers[0].URL != "https://other.example.com" {
		t.Errorf("/tickers: expected servers [{https://other.example.com}], got %v", pi.Servers)
	}
}

func TestAnalyzeInteraction_QueryParam(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/users?limit=10&offset=0",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	pi := doc.Paths["/users"]
	if pi == nil {
		t.Fatal("expected /users path")
	}
	op := pi.Get
	if op == nil {
		t.Fatal("expected GET operation")
	}

	var limitParam, offsetParam *openapi.Parameter
	for _, p := range op.Parameters {
		switch p.Value.Name {
		case "limit":
			limitParam = p.Value
		case "offset":
			offsetParam = p.Value
		}
	}
	if limitParam == nil {
		t.Error("expected limit param")
	}
	if offsetParam == nil {
		t.Error("expected offset param")
	}
	if limitParam != nil && limitParam.Schema.Value.Type != openapi.TypeInteger {
		t.Errorf("limit type: got %q, want integer", limitParam.Schema.Value.Type)
	}
}

func TestAnalyzeInteraction_CommaSeparatedParam(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/items?tags=a,b,c",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	op := doc.Paths["/items"].Get
	var tagsParam *openapi.Parameter
	for _, p := range op.Parameters {
		if p.Value.Name == "tags" {
			tagsParam = p.Value
			break
		}
	}
	if tagsParam == nil {
		t.Fatal("expected tags param")
	}
	if tagsParam.Schema.Value.Type != openapi.TypeArray {
		t.Errorf("tags type: got %q, want array", tagsParam.Schema.Value.Type)
	}
	if tagsParam.Explode == nil || *tagsParam.Explode {
		t.Error("expected explode=false for comma-separated param")
	}
}

func TestAnalyzeInteraction_BearerAuth(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodGet,
			URL:    "https://api.example.com/me",
			Headers: http.Header{
				"Authorization": {"Bearer mytoken123"},
			},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	if doc.Components.SecuritySchemes == nil {
		t.Fatal("expected security schemes")
	}
	scheme := doc.Components.SecuritySchemes["bearerAuth"]
	if scheme == nil {
		t.Fatal("expected bearerAuth security scheme")
	}
	if scheme.Value.Scheme != openapi.SecuritySchemeBearer {
		t.Errorf("scheme: got %q, want bearer", scheme.Value.Scheme)
	}

	op := doc.Paths["/me"].Get
	if len(op.Security) == 0 {
		t.Error("expected security requirement on operation")
	}
}

func TestAnalyzeInteraction_RequestBody(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodPost,
			URL:    "https://api.example.com/users",
			Headers: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: []byte(`{"name":"Alice","email":"alice@example.com"}`),
		},
		Response: cassette.Response{StatusCode: 201, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	op := doc.Paths["/users"].Post
	if op == nil {
		t.Fatal("expected POST operation")
	}
	if op.RequestBody == nil {
		t.Fatal("expected request body")
	}
	mt := op.RequestBody.Value.Content["application/json"]
	if mt == nil || mt.Schema == nil {
		t.Fatal("expected JSON schema in request body")
	}
	schema := mt.Schema.Value
	if schema.Type != openapi.TypeObject {
		t.Errorf("schema type: got %q, want object", schema.Type)
	}
	if len(schema.Properties) != 2 {
		t.Errorf("properties: got %d, want 2", len(schema.Properties))
	}
}

func TestAnalyzeInteraction_Response(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/users",
			Headers: http.Header{},
		},
		Response: cassette.Response{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": {"application/json"}},
			Body:       []byte(`[{"id":1},{"id":2}]`),
		},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	op := doc.Paths["/users"].Get
	resp := op.Responses["200"]
	if resp == nil {
		t.Fatal("expected 200 response")
	}
	mt := resp.Value.Content["application/json"]
	if mt == nil || mt.Schema == nil {
		t.Fatal("expected JSON schema in response")
	}
	if mt.Schema.Value.Type != openapi.TypeArray {
		t.Errorf("response schema type: got %q, want array", mt.Schema.Value.Type)
	}
}

func TestAnalyzeInteraction_PathParamDetection(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/users/42",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	// Should create /users/{userId}, not the literal /users/42
	if _, ok := doc.Paths["/users/42"]; ok {
		t.Error("literal /users/42 should not be in paths")
	}
	pi := doc.Paths["/users/{userId}"]
	if pi == nil {
		t.Fatalf("expected /users/{userId} path, got paths: %v", func() []string {
			var keys []string
			for k := range doc.Paths {
				keys = append(keys, string(k))
			}
			return keys
		}())
	}

	// PathItem must declare the path parameter
	var idParam *openapi.Parameter
	for _, p := range pi.Parameters {
		if p.Value.In == openapi.ParameterLocationPath && p.Value.Name == "userId" {
			idParam = p.Value
			break
		}
	}
	if idParam == nil {
		t.Fatal("expected userId path parameter on PathItem")
	}
	if !idParam.Required {
		t.Error("path parameter must be required")
	}
	if idParam.Schema == nil || idParam.Schema.Value.Type != openapi.TypeInteger {
		t.Errorf("userId schema: got %v, want integer", idParam.Schema)
	}
}

func TestAnalyzeInteraction_UUIDPathParam(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/users/550e8400-e29b-41d4-a716-446655440000",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	pi := doc.Paths["/users/{userId}"]
	if pi == nil {
		t.Fatal("expected /users/{userId} for UUID path segment")
	}

	var idParam *openapi.Parameter
	for _, p := range pi.Parameters {
		if p.Value.In == openapi.ParameterLocationPath {
			idParam = p.Value
			break
		}
	}
	if idParam == nil {
		t.Fatal("expected path parameter")
	}
	// UUID segment is not numeric, so schema should be string
	if idParam.Schema.Value.Type != openapi.TypeString {
		t.Errorf("UUID param schema: got %q, want string", idParam.Schema.Value.Type)
	}
}

func TestAnalyzeInteraction_SecondRequestReusesParametricPath(t *testing.T) {
	doc := NewDocument()

	// First request creates /users/{userId}
	ia1 := &cassette.Interaction{
		Request:  cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/users/1", Headers: http.Header{}},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia1); err != nil {
		t.Fatal(err)
	}

	// Second request with a different user ID should match the existing parametric path
	ia2 := &cassette.Interaction{
		Request:  cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/users/2", Headers: http.Header{}},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia2); err != nil {
		t.Fatal(err)
	}

	if len(doc.Paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(doc.Paths), func() []string {
			var keys []string
			for k := range doc.Paths {
				keys = append(keys, string(k))
			}
			return keys
		}())
	}
}

func TestAnalyzeInteraction_OperationID(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/users",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	op := doc.Paths["/users"].Get
	if op.OperationID != "ListUsers" {
		t.Errorf("operationId: got %q, want ListUsers", op.OperationID)
	}
}

func TestAnalyzeInteraction_CustomHeader(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodGet,
			URL:    "https://api.example.com/data",
			Headers: http.Header{
				"X-Api-Key": {"myapikey123"},
			},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	op := doc.Paths["/data"].Get
	var apiKeyParam *openapi.Parameter
	for _, p := range op.Parameters {
		if p.Value.Name == "X-Api-Key" {
			apiKeyParam = p.Value
			break
		}
	}
	if apiKeyParam == nil {
		t.Fatal("expected X-Api-Key header parameter")
	}
	if apiKeyParam.In != openapi.ParameterLocationHeader {
		t.Errorf("in: got %q, want header", apiKeyParam.In)
	}
	if !apiKeyParam.Required {
		t.Error("expected custom header to be required")
	}
}

func TestAnalyzeInteraction_BasicAuth(t *testing.T) {
	doc := NewDocument()
	// "user:pass" base64 encoded
	import64 := "dXNlcjpwYXNz"
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodGet,
			URL:    "https://api.example.com/secure",
			Headers: http.Header{
				"Authorization": {"Basic " + import64},
			},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}

	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}

	if doc.Components.SecuritySchemes["basicAuth"] == nil {
		t.Error("expected basicAuth security scheme")
	}
}

func TestAnalyzeInteraction_RequestBodyExistingPath(t *testing.T) {
	doc := NewDocument()
	// First interaction creates the path
	ia1 := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodPost,
			URL:    "https://api.example.com/items",
			Headers: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: []byte(`{"name":"item1"}`),
		},
		Response: cassette.Response{StatusCode: 201, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia1); err != nil {
		t.Fatal(err)
	}

	// Second interaction adds a new field to the existing request body
	ia2 := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodPost,
			URL:    "https://api.example.com/items",
			Headers: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: []byte(`{"name":"item2","price":9.99}`),
		},
		Response: cassette.Response{StatusCode: 201, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia2); err != nil {
		t.Fatal(err)
	}

	op := doc.Paths["/items"].Post
	rb := op.RequestBody
	if rb == nil {
		t.Fatal("expected request body")
	}
	mt := rb.Value.Content["application/json"]
	if mt == nil {
		t.Fatal("expected application/json content")
	}
}

func TestAnalyzeInteraction_MergeCustomHeaders(t *testing.T) {
	doc := NewDocument()
	// First interaction creates the header param
	ia1 := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/data",
			Headers: http.Header{"X-Api-Key": {"key1"}},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia1); err != nil {
		t.Fatal(err)
	}
	// Second interaction with same header (merge path)
	ia2 := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/data",
			Headers: http.Header{"X-Api-Key": {"key2"}},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia2); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/data"].Get
	var count int
	for _, p := range op.Parameters {
		if p.Value.Name == "X-Api-Key" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 X-Api-Key param, got %d", count)
	}
}

func TestAnalyzeInteraction_RequestBodyNewMediaType(t *testing.T) {
	doc := NewDocument()
	// First interaction creates the request body with application/json
	ia1 := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodPost,
			URL:     "https://api.example.com/upload",
			Headers: http.Header{"Content-Type": {"application/json"}},
			Body:    []byte(`{"filename":"test.txt"}`),
		},
		Response: cassette.Response{StatusCode: 201, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia1); err != nil {
		t.Fatal(err)
	}
	// Second interaction with a different media type
	ia2 := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodPost,
			URL:     "https://api.example.com/upload",
			Headers: http.Header{"Content-Type": {"application/vnd.api+json"}},
			Body:    []byte(`{"data":"stuff"}`),
		},
		Response: cassette.Response{StatusCode: 201, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia2); err != nil {
		t.Fatal(err)
	}
	rb := doc.Paths["/upload"].Post.RequestBody
	if len(rb.Value.Content) < 2 {
		t.Errorf("expected at least 2 content types, got %d", len(rb.Value.Content))
	}
}

func TestIsCustomHeader(t *testing.T) {
	custom := []string{"x-api-key", "x-request-id", "api-version", "api-key"}
	for _, h := range custom {
		if !isCustomHeader(h) {
			t.Errorf("expected %q to be custom header", h)
		}
	}
	notCustom := []string{"Accept", "Host", "Connection", "Cache-Control", "Accept-Language"}
	for _, h := range notCustom {
		if isCustomHeader(h) {
			t.Errorf("expected %q NOT to be custom header", h)
		}
	}
}

func TestAnalyzeInteraction_ContentTypeWithCharset(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodPost,
			URL:    "https://api.example.com/data",
			Headers: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: []byte(`{"key":"value"}`),
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/data"].Post
	if op.RequestBody == nil {
		t.Error("expected request body")
	}
}

func TestAnalyzeInteraction_CommaSeparatedIntParams(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/items?ids=1,2,3",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/items"].Get
	var idsParam *openapi.Parameter
	for _, p := range op.Parameters {
		if p.Value.Name == "ids" {
			idsParam = p.Value
			break
		}
	}
	if idsParam == nil {
		t.Fatal("expected ids param")
	}
	if idsParam.Schema.Value.Type != openapi.TypeArray {
		t.Errorf("type: got %q, want array", idsParam.Schema.Value.Type)
	}
	if idsParam.Schema.Value.Items.Value.Type != openapi.TypeInteger {
		t.Errorf("items type: got %q, want integer", idsParam.Schema.Value.Items.Value.Type)
	}
}

func TestAnalyzeInteraction_IgnoredHeaders(t *testing.T) {
	doc := NewDocument()
	ia := &cassette.Interaction{
		Request: cassette.Request{
			Method: http.MethodGet,
			URL:    "https://api.example.com/data",
			Headers: http.Header{
				"User-Agent": {"Mozilla/5.0"},
				"Referer":    {"https://example.com"},
				"Cookie":     {"session=abc"},
				"Accept":     {"application/json"},
			},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/data"].Get
	if len(op.Parameters) != 0 {
		t.Errorf("expected no parameters from ignored headers, got %d", len(op.Parameters))
	}
}

func TestAnalyzeInteraction_MergeQueryParams(t *testing.T) {
	doc := NewDocument()
	// First interaction
	ia1 := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/search?q=hello",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia1); err != nil {
		t.Fatal(err)
	}

	// Second interaction with same param
	ia2 := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/search?q=world",
			Headers: http.Header{},
		},
		Response: cassette.Response{StatusCode: http.StatusOK, Headers: http.Header{}},
	}
	if err := analyzeInteraction(doc, ia2); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/search"].Get
	var count int
	for _, p := range op.Parameters {
		if p.Value.Name == "q" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 q param, got %d", count)
	}
}

func TestAnalyzeInteraction_MergeResponses(t *testing.T) {
	doc := NewDocument()

	// First interaction: response with field "id"
	ia1 := &cassette.Interaction{
		Request: cassette.Request{Method: http.MethodGet, URL: "https://api.example.com/users/1", Headers: http.Header{}},
		Response: cassette.Response{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": {"application/json"}},
			Body:       []byte(`{"id":1,"name":"Alice"}`),
		},
	}
	if err := analyzeInteraction(doc, ia1); err != nil {
		t.Fatal(err)
	}

	// Second interaction: same path, same status
	ia2 := &cassette.Interaction{
		Request: cassette.Request{
			Method:  http.MethodGet,
			URL:     "https://api.example.com/users/2",
			Headers: http.Header{},
		},
		Response: cassette.Response{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": {"application/json"}},
			Body:       []byte(`{"id":2,"name":"Bob","email":"bob@example.com"}`),
		},
	}
	if err := analyzeInteraction(doc, ia2); err != nil {
		t.Fatal(err)
	}

	// Should have a single path with a merged schema
	var matched *openapi.PathItem
	for _, pi := range doc.Paths {
		matched = pi
	}
	if matched == nil {
		t.Fatal("expected a path item")
	}
	op := matched.Get
	if op == nil {
		t.Fatal("expected GET operation")
	}
	resp := op.Responses["200"]
	if resp == nil {
		t.Fatal("expected 200 response")
	}
}
