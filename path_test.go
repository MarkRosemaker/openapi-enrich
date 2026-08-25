package enrich

import (
	"net/url"
	"testing"

	"github.com/MarkRosemaker/openapi"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		input      string
		wantLen    int
		wantParam  []bool
		wantName   []string
		wantPrefix []string
		wantSuffix []string
	}{
		{"/users", 1, []bool{false}, []string{"users"}, []string{""}, []string{""}},
		{"/users/{id}", 2, []bool{false, true}, []string{"users", "id"}, []string{"", ""}, []string{"", ""}},
		{
			"/users/{userId}/posts/{postId}", 4,
			[]bool{false, true, false, true},
			[]string{"users", "userId", "posts", "postId"},
			[]string{"", "", "", ""},
			[]string{"", "", "", ""},
		},
		{"/", 1, []bool{false}, []string{""}, []string{""}, []string{""}},
		// Embedded param: CIK{cik}.json
		{
			"/companyfacts/CIK{cik}.json", 2,
			[]bool{false, true},
			[]string{"companyfacts", "cik"},
			[]string{"", "CIK"},
			[]string{"", ".json"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			pp := parsePath(tc.input)
			if len(pp) != tc.wantLen {
				t.Fatalf("len=%d, want %d", len(pp), tc.wantLen)
			}
			for i, el := range pp {
				if el.isParam != tc.wantParam[i] {
					t.Errorf("[%d] isParam=%v, want %v", i, el.isParam, tc.wantParam[i])
				}
				if el.name != tc.wantName[i] {
					t.Errorf("[%d] name=%q, want %q", i, el.name, tc.wantName[i])
				}
				if el.prefix != tc.wantPrefix[i] {
					t.Errorf("[%d] prefix=%q, want %q", i, el.prefix, tc.wantPrefix[i])
				}
				if el.suffix != tc.wantSuffix[i] {
					t.Errorf("[%d] suffix=%q, want %q", i, el.suffix, tc.wantSuffix[i])
				}
			}
		})
	}
}

func TestParsedPathFits(t *testing.T) {
	pp := parsePath("/users/{id}")

	fits := [][]string{
		{"users", "123"},
		{"users", "abc"},
		{"users", "550e8400-e29b-41d4-a716-446655440000"},
		// Non-ID slug with extra segments: greedy absorption is allowed because
		// the first consumed segment is not a UUID/integer.
		{"users", "abc", "extra"},
	}
	for _, segs := range fits {
		if !pp.fits(segs) {
			t.Errorf("expected %v to fit /users/{id}", segs)
		}
	}

	noFit := [][]string{
		{"posts", "123"},
		{"users"},
		// An integer or UUID followed by extra segments should NOT match greedily:
		// those extra segments belong to a more-specific sub-route.
		{"users", "123", "extra"},
		{"users", "550e8400-e29b-41d4-a716-446655440000", "extra"},
	}
	for _, segs := range noFit {
		if pp.fits(segs) {
			t.Errorf("expected %v NOT to fit /users/{id}", segs)
		}
	}
}

func TestParsedPathFits_GreedyPackagePath(t *testing.T) {
	pp := parsePath("/package/{path}")

	fits := [][]string{
		{"package", "github.com"},
		{"package", "github.com", "google", "go-cmp", "cmp"},
	}
	for _, segs := range fits {
		if !pp.fits(segs) {
			t.Errorf("expected %v to fit /package/{path}", segs)
		}
	}

	noFit := [][]string{
		{"other", "github.com", "google", "go-cmp", "cmp"},
		{"package"},
	}
	for _, segs := range noFit {
		if pp.fits(segs) {
			t.Errorf("expected %v NOT to fit /package/{path}", segs)
		}
	}
}

func TestFindPathItem_ExactMatch(t *testing.T) {
	doc := NewDocument()
	doc.Servers = openapi.Servers{{URL: "https://api.example.com"}}
	pi := &openapi.PathItem{}
	doc.Paths = openapi.Paths{}
	doc.Paths.Set("/users", pi)

	reqURL, _ := url.Parse("https://api.example.com/users")
	gotPath, gotPI := findPathItem(doc, reqURL)

	if gotPath != "/users" {
		t.Errorf("path: got %q, want /users", gotPath)
	}
	if gotPI != pi {
		t.Error("expected exact PathItem match")
	}
}

func TestFindPathItem_ParametricMatch(t *testing.T) {
	doc := NewDocument()
	doc.Servers = openapi.Servers{{URL: "https://api.example.com"}}
	pi := &openapi.PathItem{}
	doc.Paths = openapi.Paths{}
	doc.Paths.Set("/users/{id}", pi)

	reqURL, _ := url.Parse("https://api.example.com/users/42")
	gotPath, gotPI := findPathItem(doc, reqURL)

	if gotPath != "/users/{id}" {
		t.Errorf("path: got %q, want /users/{id}", gotPath)
	}
	if gotPI != pi {
		t.Error("expected parametric PathItem match")
	}
}

func TestFindPathItem_NoMatch(t *testing.T) {
	doc := NewDocument()
	doc.Servers = openapi.Servers{{URL: "https://api.example.com"}}
	doc.Paths = openapi.Paths{}

	reqURL, _ := url.Parse("https://api.example.com/users/42")
	gotPath, gotPI := findPathItem(doc, reqURL)

	if gotPath != "/users/42" {
		t.Errorf("path: got %q, want /users/42", gotPath)
	}
	if gotPI != nil {
		t.Error("expected nil PathItem for no match")
	}
}

func TestExtractEmbeddedParam(t *testing.T) {
	tests := []struct {
		seg        string
		wantPrefix string
		wantParam  string
		wantSuffix string
		wantOK     bool
	}{
		{"CIK0000320193.json", "CIK", "cik", ".json", true},
		{"CIK0001652044.json", "CIK", "cik", ".json", true},
		{"ABC12345", "ABC", "abc", "", true},
		{"v2", "", "", "", false},     // only 1 digit, not 4+
		{"42", "", "", "", false},     // no alpha prefix
		{"users", "", "", "", false},  // no digits
		{"CIK123", "", "", "", false}, // only 3 digits
		{"", "", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.seg, func(t *testing.T) {
			prefix, paramName, suffix, ok := extractEmbeddedParam(tc.seg)
			if ok != tc.wantOK {
				t.Errorf("ok=%v, want %v", ok, tc.wantOK)
			}
			if prefix != tc.wantPrefix {
				t.Errorf("prefix=%q, want %q", prefix, tc.wantPrefix)
			}
			if paramName != tc.wantParam {
				t.Errorf("paramName=%q, want %q", paramName, tc.wantParam)
			}
			if suffix != tc.wantSuffix {
				t.Errorf("suffix=%q, want %q", suffix, tc.wantSuffix)
			}
		})
	}
}

func TestParsedPathFits_EmbeddedParam(t *testing.T) {
	pp := parsePath("/companyfacts/CIK{cik}.json")

	fits := [][]string{
		{"companyfacts", "CIK0000320193.json"},
		{"companyfacts", "CIK0001652044.json"},
		{"companyfacts", "CIK9999999999.json"},
	}
	for _, segs := range fits {
		if !pp.fits(segs) {
			t.Errorf("expected %v to fit /companyfacts/CIK{cik}.json", segs)
		}
	}

	noFit := [][]string{
		{"companyfacts", "OTHER0000320193.json"},
		{"companyfacts", "CIK0000320193.csv"},
		{"companyfacts", "CIK.json"},
		{"xbrl", "CIK0000320193.json"},
	}
	for _, segs := range noFit {
		if pp.fits(segs) {
			t.Errorf("expected %v NOT to fit /companyfacts/CIK{cik}.json", segs)
		}
	}
}

func TestNewParametricPath(t *testing.T) {
	tests := []struct {
		urlPath  string
		wantPath openapi.Path
	}{
		{"/users/42", "/users/{userId}"},
		{"/users/abc", "/users/abc"},
		{"/posts/550e8400-e29b-41d4-a716-446655440000", "/posts/{postId}"},
		{"/users", "/users"},
		{"/api/xbrl/companyfacts/CIK0000320193.json", "/api/xbrl/companyfacts/CIK{cik}.json"},
	}

	for _, tc := range tests {
		t.Run(tc.urlPath, func(t *testing.T) {
			got, _ := newParametricPath(tc.urlPath)
			if got != tc.wantPath {
				t.Errorf("got %q, want %q", got, tc.wantPath)
			}
		})
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct{ input, want string }{
		{"users", "user"},
		{"posts", "post"},
		{"categories", "category"},
		{"boxes", "box"},
		{"user", "user"},
	}
	for _, tc := range tests {
		if got := singularize(tc.input); got != tc.want {
			t.Errorf("singularize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFindPathItem_NoServers(t *testing.T) {
	doc := NewDocument()
	doc.Paths = openapi.Paths{}
	reqURL, _ := url.Parse("https://api.example.com/users")
	gotPath, gotPI := findPathItem(doc, reqURL)
	if gotPath != "" {
		t.Errorf("expected empty path when no servers, got %q", gotPath)
	}
	if gotPI != nil {
		t.Error("expected nil PathItem when no servers")
	}
}

func TestPathSegments_Empty(t *testing.T) {
	segs := pathSegments("/")
	if len(segs) != 1 || segs[0] != "" {
		t.Errorf("expected [\"\"], got %v", segs)
	}
}

func TestLooksLikeID_Empty(t *testing.T) {
	if looksLikeID("") {
		t.Error("empty string should not look like ID")
	}
}

func TestDeriveParamName_FirstSegment(t *testing.T) {
	name := deriveParamName([]string{"123"}, 0)
	if name != "id" {
		t.Errorf("got %q, want id", name)
	}
}

func TestRelativePath(t *testing.T) {
	tests := []struct {
		reqPath     string
		serverURL   string
		wantRelPath string
	}{
		{"https://api.example.com/v1/users", "https://api.example.com/v1", "/users"},
		{"https://api.example.com/users", "https://api.example.com", "/users"},
		{"https://api.example.com/", "https://api.example.com", "/"},
		{"https://api.example.com/users", "://bad-url", "/users"}, // bad server URL falls back
	}
	for _, tc := range tests {
		reqURL, _ := url.Parse(tc.reqPath)
		got := relativePath(reqURL, tc.serverURL)
		if got != tc.wantRelPath {
			t.Errorf("relativePath(%q, %q) = %q, want %q",
				tc.reqPath, tc.serverURL, got, tc.wantRelPath)
		}
	}
}
