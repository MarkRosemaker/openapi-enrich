// Package migrate provides helpers for converting cassette files from
// third-party recording libraries into the native [cassette.Interactions] format.
package migrate

import (
	"strings"

	"github.com/MarkRosemaker/openapi-enrich/cassette"
	govcr "gopkg.in/dnaeon/go-vcr.v3/cassette"
)

// FromGoVCRFile reads a go-vcr cassette YAML file using go-vcr's native
// loader and converts it to [cassette.Interactions].
//
// path is the cassette name passed to [gopkg.in/dnaeon/go-vcr.v3/cassette.Load];
// the ".yaml" extension is stripped automatically if present, matching go-vcr's
// own convention.
func FromGoVCRFile(path string) (cassette.Interactions, error) {
	name := strings.TrimSuffix(path, ".yaml")

	c, err := govcr.Load(name)
	if err != nil {
		return nil, err
	}

	out := make(cassette.Interactions, len(c.Interactions))
	for i, ia := range c.Interactions {
		out[i] = cassette.Interaction{
			Request: cassette.Request{
				Method:  ia.Request.Method,
				URL:     ia.Request.URL,
				Headers: nilIfEmpty(ia.Request.Headers),
				Body:    bodyFromString(ia.Request.Body),
			},
			Response: cassette.Response{
				StatusCode: ia.Response.Code,
				Headers:    nilIfEmpty(ia.Response.Headers),
				Body:       bodyFromString(ia.Response.Body),
			},
		}
	}

	return out, nil
}

// bodyFromString converts a go-vcr body string to a [cassette.Body].
// An empty string produces a nil body (omitted in JSON output).
func bodyFromString(s string) cassette.Body {
	if s == "" {
		return nil
	}

	return cassette.Body(s)
}

// nilIfEmpty returns nil when h has no entries so that omitempty works correctly.
func nilIfEmpty[M ~map[K]V, K comparable, V any](m M) M {
	if len(m) == 0 {
		return nil
	}

	return m
}
