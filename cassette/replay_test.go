package cassette_test

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

// recorded is a recording of a call to rawURL with body, answered with status and answer.
func recorded(t *testing.T, method, rawURL, body string, status int, answer string) *cassette.Interaction {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}

	return &cassette.Interaction{
		Request:  cassette.Request{Method: method, URL: cassette.NormalizeURL(*u), Body: cassette.Body(body)},
		Response: cassette.Response{StatusCode: status, Body: cassette.Body(answer)},
	}
}

// call sends method to rawURL with body and headers through rt and returns the body it is answered with.
func call(t *testing.T, rt http.RoundTripper, method, rawURL, body string, headers http.Header) (string, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, rawURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	if body == "" {
		req.Body = http.NoBody
	}

	req.Header = headers
	if req.Header == nil {
		req.Header = http.Header{}
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rsp, err := rt.RoundTrip(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	out, err := io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatal(err)
	}

	return string(out), nil
}

func TestReplay(t *testing.T) {
	t.Parallel()

	const api = "https://api.example.com"

	type request struct {
		method, url, body string
		headers           http.Header
		want              string // the answer, or the error's text
	}

	for name, tc := range map[string]struct {
		recorded []*cassette.Interaction
		opts     []cassette.Option
		calls    []request
		unused   int
	}{
		"query order and JSON member order do not matter": {
			recorded: []*cassette.Interaction{recorded(t, "POST", api+"/a?x=1&ids=1,2", `{"a":1,"b":2}`, 200, `"ok"`)},
			calls:    []request{{method: "POST", url: api + "/a?ids=1,2&x=1", body: `{ "b": 2, "a": 1 }`, want: `"ok"`}},
		},
		"headers are not compared": {
			recorded: []*cassette.Interaction{recorded(t, "GET", api+"/a", "", 200, `"ok"`)},
			calls:    []request{{method: "GET", url: api + "/a", headers: http.Header{"X-Client": {"another app"}}, want: `"ok"`}},
		},
		"a request takes its recordings in turn, then the last again": {
			recorded: []*cassette.Interaction{
				recorded(t, "GET", api+"/n", "", 200, `1`),
				recorded(t, "GET", api+"/other", "", 200, `"other"`),
				recorded(t, "GET", api+"/n", "", 200, `2`),
			},
			calls: []request{
				{method: "GET", url: api + "/n", want: `1`},
				{method: "GET", url: api + "/n", want: `2`},
				{method: "GET", url: api + "/n", want: `2`},
			},
			unused: 1,
		},
		"a request without a recording fails": {
			recorded: []*cassette.Interaction{recorded(t, "GET", api+"/a", "", 200, `"ok"`)},
			calls:    []request{{method: "GET", url: api + "/b", want: "GET https://api.example.com/b: no recorded interaction"}},
			unused:   1,
		},
		"in order, a request out of turn fails, saying why": {
			recorded: []*cassette.Interaction{
				recorded(t, "GET", api+"/a", "", 200, `"a"`),
				recorded(t, "GET", api+"/b", "", 200, `"b"`),
			},
			opts: []cassette.Option{cassette.InOrder()},
			calls: []request{
				{method: "GET", url: api + "/b", want: "interaction #0: got URL https://api.example.com/b, want https://api.example.com/a"},
				{method: "GET", url: api + "/a", want: `"a"`},
				{method: "GET", url: api + "/b", want: `"b"`},
				{method: "GET", url: api + "/b", want: "unexpected request GET https://api.example.com/b: no recorded interaction"},
			},
		},
		"compared headers are exact, Authorization by its scheme": {
			recorded: []*cassette.Interaction{func() *cassette.Interaction {
				ia := recorded(t, "GET", api+"/a", "", 200, `"ok"`)
				ia.Request.Headers = http.Header{"Authorization": {"Bearer ****"}, "X-Client": {"lib"}, "User-Agent": {"recorder"}}

				return ia
			}()},
			opts: []cassette.Option{cassette.WithMatcher(cassette.MatchRequest(cassette.CompareHeaders("User-Agent")))},
			calls: []request{
				{method: "GET", url: api + "/a", headers: http.Header{"Authorization": {"Basic dXNlcg=="}, "X-Client": {"lib"}}, want: `GET https://api.example.com/a: no recorded interaction`},
				{method: "GET", url: api + "/a", headers: http.Header{"Authorization": {"Bearer real"}, "X-Client": {"other"}}, want: `GET https://api.example.com/a: no recorded interaction`},
				{method: "GET", url: api + "/a", headers: http.Header{"Authorization": {"Bearer real"}, "X-Client": {"lib"}, "User-Agent": {"app"}}, want: `"ok"`},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := cassette.Replay(tc.recorded, tc.opts...)

			for i, c := range tc.calls {
				got, err := call(t, rec, c.method, c.url, c.body, c.headers)
				if err != nil {
					got = err.Error()
				}

				if got != c.want {
					t.Errorf("call %d: got %s, want %s", i, got, c.want)
				}
			}

			if got := len(rec.Unused()); got != tc.unused {
				t.Errorf("got %d unused, want %d", got, tc.unused)
			}
		})
	}
}

func TestReplay_ErrNoInteraction(t *testing.T) {
	t.Parallel()

	_, err := call(t, cassette.Replay(nil), "GET", "https://api.example.com/a", "", nil)
	if !errors.Is(err, cassette.ErrNoInteraction) {
		t.Fatalf("got %v, want ErrNoInteraction", err)
	}
}

func TestReplay_OptionalAuthorization(t *testing.T) {
	t.Parallel()

	ia := recorded(t, "GET", "https://api.example.com/feed", "", 200, `"public"`)
	optional := func(r *http.Request) bool { return r.Header.Get("X-Optional") != "" }
	rec := cassette.Replay(cassette.Interactions{ia},
		cassette.WithMatcher(cassette.MatchRequest(cassette.CompareHeaders("X-Optional"), cassette.OptionalAuthorization(optional))))

	// recorded without credentials, a call that may send them matches; one that must not, does not
	if _, err := call(t, rec, "GET", "https://api.example.com/feed", "", http.Header{"Authorization": {"Bearer x"}}); err == nil {
		t.Error("credentials where none were recorded: got a match")
	}

	if _, err := call(t, rec, "GET", "https://api.example.com/feed", "", http.Header{"Authorization": {"Bearer x"}, "X-Optional": {"1"}}); err != nil {
		t.Error(err)
	}
}

func TestReplay_RecordMisses(t *testing.T) {
	t.Parallel()

	sent := 0
	server := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sent++
		return &http.Response{StatusCode: 201, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`"new"`))}, nil
	})

	rec := cassette.Replay(cassette.Interactions{recorded(t, "GET", "https://api.example.com/old", "", 200, `"old"`)}, cassette.RecordMisses(server))

	for _, c := range []struct{ path, want string }{{"/old", `"old"`}, {"/new?b=2&a=1", `"new"`}} {
		if got, err := call(t, rec, "GET", "https://api.example.com"+c.path, "", nil); err != nil || got != c.want {
			t.Fatalf("%s: got %s, %v, want %s", c.path, got, err, c.want)
		}
	}

	ias := rec.Interactions()
	if sent != 1 || len(ias) != 2 || ias[1].Response.StatusCode != 201 || ias[1].Request.URL.RawQuery != "a=1&b=2" {
		t.Fatalf("got %d sent and %d interactions, want the miss sent once and recorded with its query sorted", sent, len(ias))
	}
}

func TestReplay_Concurrent(t *testing.T) {
	t.Parallel()

	rec := cassette.Replay(cassette.Interactions{recorded(t, "GET", "https://api.example.com/a", "", 200, `"ok"`)})

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := call(t, rec, "GET", "https://api.example.com/a", "", nil); err != nil {
				t.Error(err)
			}
		})
	}

	wg.Wait()

	if got := len(rec.Interactions()); got != 20 {
		t.Errorf("got %d interactions, want 20", got)
	}
}

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	// sorted by name, each kept as written: a delimiter is no escaped comma, and values of one name keep their order
	u, _ := url.Parse("https://x.example/p?ids=1,2&b=2&a=x%2Cy&b=1")
	if got := cassette.NormalizeURL(*u); got.RawQuery != "a=x%2Cy&b=2&b=1&ids=1,2" {
		t.Errorf("got %s", got.RawQuery)
	}
}

func TestInteractions_URL(t *testing.T) {
	t.Parallel()

	ias, err := cassette.InteractionsUnmarshal([]byte(`[{"request":{"method":"GET","url":"https://x.example/p?b=1&a=2"},"response":{"statusCode":200}},{"request":{"method":"","url":""}}]`))
	if err != nil {
		t.Fatal(err)
	}

	// read normalized; a scaffold has no URL
	if got := ias[0].Request.URL.String(); got != "https://x.example/p?a=2&b=1" || !ias[1].Request.IsScaffold() {
		t.Errorf("got %s and scaffold %t", got, ias[1].Request.IsScaffold())
	}

	if _, err := cassette.InteractionsUnmarshal([]byte(`[{"request":{"method":"GET","url":"://bad"}}]`)); err == nil {
		t.Error("a URL that does not parse: got no error")
	}
}

func TestWriteFile_Masks(t *testing.T) {
	t.Parallel()

	ia := recorded(t, "GET", "https://api.example.com/a?api_key=secret&q=go", "", 200, `{"token":"abc"}`)
	ia.Request.Headers = http.Header{"Authorization": {"Bearer secret"}, "X-Client": {"my-app"}}
	path := filepath.Join(t.TempDir(), "ias.json")

	if err := cassette.DefaultMasker().Keep("X-Client").WriteFile(path, cassette.Interactions{ia}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{"secret", `"abc"`} {
		if strings.Contains(string(data), secret) {
			t.Errorf("written: %s", secret)
		}
	}

	if !strings.Contains(string(data), "api_key=******") || !strings.Contains(string(data), "my-app") || !strings.Contains(string(data), "q=go") {
		t.Errorf("got %s, want the key masked, the kept header and the query kept", data)
	}

	if ia.Request.Headers.Get("Authorization") != "Bearer secret" {
		t.Error("writing masked what was written")
	}

	// masking what is masked changes nothing
	written, err := cassette.InteractionsReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := cassette.DefaultMasker().Keep("X-Client").WriteFile(path, written); err != nil {
		t.Fatal(err)
	}

	if again, _ := os.ReadFile(path); string(again) != string(data) {
		t.Errorf("masking twice: got %s, want %s", again, data)
	}
}

func TestAddInteraction_OncePerCall(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "ias.json")
	failure := func(status int, body string) *cassette.Interaction {
		ia := recorded(t, "GET", "https://api.example.com/a", "", status, body)
		ia.Response.Headers = http.Header{"Date": {time()}}

		return ia
	}

	// the same call again, even at another time, is the same failure; another status or body is another
	for _, ia := range []*cassette.Interaction{failure(502, "down"), failure(502, "down"), failure(500, "down"), failure(502, "gone")} {
		if err := cassette.AddInteraction(path, ia); err != nil {
			t.Fatal(err)
		}
	}

	ias, err := cassette.InteractionsReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(ias) != 3 {
		t.Fatalf("got %d interactions, want 3", len(ias))
	}
}

var (
	clockMu sync.Mutex
	clock   int
)

// time is a different date on every call.
func time() string {
	clockMu.Lock()
	defer clockMu.Unlock()

	clock++

	return strings.Repeat("x", clock)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
