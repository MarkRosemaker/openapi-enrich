package recorder

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

// errReader is an io.Reader that always returns an error.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read error") }

// roundTripFunc is a convenient http.RoundTripper backed by a function.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeTransport(statusCode int, body string, headers http.Header) http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		h := headers.Clone()
		if h == nil {
			h = http.Header{}
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     h,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	})
}

func TestRecordingTransport_RecordsInteraction(t *testing.T) {
	rt := &Transport{
		Transport: fakeTransport(200, `{"ok":true}`,
			http.Header{"Content-Type": {"application/json"}}),
	}
	client := &http.Client{Transport: rt}

	resp, err := client.Get("https://api.example.com/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if len(rt.Interactions) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(rt.Interactions))
	}
	ia := rt.Interactions[0]
	if ia.Request.Method != http.MethodGet {
		t.Errorf("method: got %q, want GET", ia.Request.Method)
	}
	if ia.Request.URL != "https://api.example.com/users" {
		t.Errorf("URL: got %q", ia.Request.URL)
	}
	if ia.Response.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", ia.Response.StatusCode)
	}
	if string(ia.Response.Body) != `{"ok":true}` {
		t.Errorf("body: got %q", ia.Response.Body)
	}
}

func TestRecordingTransport_RequestBodyRecorded(t *testing.T) {
	rt := &Transport{
		Transport: fakeTransport(201, `{"id":1}`,
			http.Header{"Content-Type": {"application/json"}}),
	}
	client := &http.Client{Transport: rt}

	reqBody := []byte(`{"name":"Alice"}`)
	resp, err := client.Post("https://api.example.com/users",
		"application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	ia := rt.Interactions[0]
	if !bytes.Equal(ia.Request.Body, reqBody) {
		t.Errorf("request body: got %q, want %q", ia.Request.Body, reqBody)
	}
}

func TestRecordingTransport_CallerCanStillReadBody(t *testing.T) {
	rt := &Transport{
		Transport: fakeTransport(200, `hello`, nil),
	}
	client := &http.Client{Transport: rt}

	resp, err := client.Get("https://api.example.com/ping")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("caller body: got %q, want %q", got, "hello")
	}
}

func TestRecordingTransport_ReplaysCachedResponse(t *testing.T) {
	// The underlying transport counts how many real calls are made.
	calls := 0
	rt := &Transport{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"n":1}`)),
			}, nil
		}),
	}
	client := &http.Client{Transport: rt}

	// Three identical requests.
	for i := range 3 {
		resp, err := client.Get("https://api.example.com/users")
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		resp.Body.Close()
	}

	// Only one real network call should have been made.
	if calls != 1 {
		t.Errorf("underlying transport called %d times, want 1", calls)
	}
	// And one Interaction.
	if len(rt.Interactions) != 1 {
		t.Fatalf("got %d interactions, want 1", len(rt.Interactions))
	}

	ia := rt.Interactions[0]
	if string(ia.Response.Body) != `{"n":1}` {
		t.Errorf("body: got %q", ia.Response.Body)
	}
}

func TestRecordingTransport_DifferentURLsNotCached(t *testing.T) {
	calls := 0
	rt := &Transport{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		}),
	}
	client := &http.Client{Transport: rt}

	if _, err := client.Get("https://api.example.com/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get("https://api.example.com/b"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get("https://api.example.com/a"); err != nil { // replay
		t.Fatal(err)
	}

	if calls != 2 {
		t.Errorf("underlying transport called %d times, want 2", calls)
	}
	if len(rt.Interactions) != 2 {
		t.Errorf("got %d interactions, want 2", len(rt.Interactions))
	}
}

func TestRecordingTransport_DifferentBodiesNotCached(t *testing.T) {
	calls := 0
	rt := &Transport{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: 201,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		}),
	}
	client := &http.Client{Transport: rt}

	if _, err := client.Post("https://api.example.com/items", "application/json", bytes.NewReader([]byte(`{"a":1}`))); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Post("https://api.example.com/items", "application/json", bytes.NewReader([]byte(`{"b":2}`))); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Post("https://api.example.com/items", "application/json", bytes.NewReader([]byte(`{"a":1}`))); err != nil { // replay
		t.Fatal(err)
	}

	if calls != 2 {
		t.Errorf("underlying transport called %d times, want 2", calls)
	}
	if len(rt.Interactions) != 2 {
		t.Errorf("got %d interactions, want 2", len(rt.Interactions))
	}
}

func TestRecordingTransport_DifferentHeadersSameURL_Cached(t *testing.T) {
	// Headers are NOT part of the cache key — same method+URL+body
	// returns cached response even with different auth headers.
	calls := 0
	rt := &Transport{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		}),
	}

	req1, _ := http.NewRequest(http.MethodGet, "https://api.example.com/me", nil)
	req1.Header.Set("Foo", "Bar")
	req2, _ := http.NewRequest(http.MethodGet, "https://api.example.com/me", nil)
	req2.Header.Set("Foo", "Baz")

	rt.RoundTrip(req1) //nolint
	rt.RoundTrip(req2) //nolint

	if calls != 2 {
		t.Errorf("underlying transport called %d times, want 2", calls)
	}
	if len(rt.Interactions) != 2 {
		t.Fatalf("got %d interactions, want 2", len(rt.Interactions))
	}

	if rt.Interactions[1].Request.Headers.Get("Foo") != "Baz" {
		t.Error("second interaction should record its own request headers")
	}
}

func TestRecordingTransport_ReplayedBodyReadableByCallerEachTime(t *testing.T) {
	rt := &Transport{
		Transport: fakeTransport(200, `data`, nil),
	}
	client := &http.Client{Transport: rt}

	for i := range 3 {
		resp, err := client.Get("https://api.example.com/data")
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(got) != "data" {
			t.Errorf("call %d: body got %q, want %q", i, got, "data")
		}
	}
}

func TestRecordingTransport_MultipleRequests(t *testing.T) {
	rt := &Transport{
		Transport: fakeTransport(200, `{}`,
			http.Header{"Content-Type": {"application/json"}}),
	}
	client := &http.Client{Transport: rt}

	urls := []string{
		"https://api.example.com/a",
		"https://api.example.com/b",
		"https://api.example.com/c",
	}
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			t.Fatalf("GET %s: %v", u, err)
		}
		resp.Body.Close()
	}

	if len(rt.Interactions) != len(urls) {
		t.Errorf("got %d interactions, want %d", len(rt.Interactions), len(urls))
	}
	for i, ia := range rt.Interactions {
		if ia.Request.URL != urls[i] {
			t.Errorf("[%d] URL: got %q, want %q", i, ia.Request.URL, urls[i])
		}
	}
}

func TestRecordingTransport_DefaultTransport(t *testing.T) {
	rt := &Transport{}
	if _, ok := rt.underlying().(*defaultOrInsecureTransport); !ok {
		t.Error("expected underlying() to return *defaultOrInsecureTransport when Transport is nil")
	}
}

func TestRecordingTransport_UnderlyingError(t *testing.T) {
	rt := &Transport{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("dial refused")
		}),
	}
	client := &http.Client{Transport: rt}

	_, err := client.Get("https://api.example.com/err")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(rt.Interactions) != 0 {
		t.Error("expected no interaction recorded on transport error")
	}
}

func TestRecordingTransport_RequestBodyError(t *testing.T) {
	rt := &Transport{Transport: fakeTransport(200, `{}`, nil)}
	req, _ := http.NewRequest(http.MethodPost, "https://api.example.com/users",
		io.NopCloser(errReader{}))
	_, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error from request body read failure")
	}
}

func TestRecordingTransport_ResponseBodyError(t *testing.T) {
	rt := &Transport{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(errReader{}),
			}, nil
		}),
	}
	client := &http.Client{Transport: rt}

	_, err := client.Get("https://api.example.com/badbody")
	if err == nil {
		t.Fatal("expected error from body read failure")
	}
}

func TestRecordingTransport_RequestHeadersRecorded(t *testing.T) {
	rt := &Transport{Transport: fakeTransport(200, `{}`, nil)}
	client := &http.Client{Transport: rt}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/me", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if rt.Interactions[0].Request.Headers.Get("Authorization") != "Bearer secret" {
		t.Errorf("Authorization header not recorded: %v", rt.Interactions[0].Request.Headers)
	}
}

func TestRequestKey(t *testing.T) {
	// Same method+URL, empty body → same key.
	k1 := requestKey(cassette.Request{
		Method: http.MethodGet,
		URL:    "https://api.example.com/users",
	})
	k2 := requestKey(cassette.Request{
		Method: http.MethodGet,
		URL:    "https://api.example.com/users",
	})
	if k1 != k2 {
		t.Errorf("nil and empty body should produce the same key: %q vs %q", k1, k2)
	}

	// Different URLs → different keys.
	k3 := requestKey(cassette.Request{
		Method: http.MethodGet,
		URL:    "https://api.example.com/posts",
	})
	if k1 == k3 {
		t.Error("different URLs should produce different keys")
	}

	// Same URL, different body → different keys.
	k4 := requestKey(cassette.Request{
		Method: http.MethodPost,
		URL:    "https://api.example.com/users",
		Body:   []byte(`{"a":1}`),
	})
	k5 := requestKey(cassette.Request{
		Method: http.MethodPost,
		URL:    "https://api.example.com/users",
		Body:   []byte(`{"b":2}`),
	})
	if k4 == k5 {
		t.Error("different bodies should produce different keys")
	}

	// Same URL + body, different method → different keys.
	k6 := requestKey(cassette.Request{
		Method: http.MethodDelete,
		URL:    "https://api.example.com/users",
	})
	if k1 == k6 {
		t.Error("different methods should produce different keys")
	}

	// Different query parameter order → same key.
	k7 := requestKey(cassette.Request{
		Method: http.MethodGet,
		URL:    "https://api.example.com/users?foo=bar&baz=quux",
	})
	k8 := requestKey(cassette.Request{
		Method: http.MethodGet,
		URL:    "https://api.example.com/users?baz=quux&foo=bar",
	})
	if k7 != k8 {
		t.Errorf("different param order should produce the same key: %q vs %q", k7, k8)
	}
}
