package cassette_test

import (
	"testing"

	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestTrimResponseBodies(t *testing.T) {
	t.Parallel()

	ias := cassette.Interactions{
		{Response: cassette.Response{Body: cassette.Body(`[1,2,3,4,5]`)}},
		{Response: cassette.Response{Body: cassette.Body(`not json`)}},
		{Response: cassette.Response{}},
	}

	ias.TrimResponseBodies(2)

	if got, want := string(ias[0].Response.Body), `[1,2]`; got != want {
		t.Errorf("body 0 = %s, want %s", got, want)
	}

	if got, want := string(ias[1].Response.Body), `not json`; got != want {
		t.Errorf("non-JSON body should be left alone, got %s, want %s", got, want)
	}

	if ias[2].Response.Body != nil {
		t.Errorf("empty body should stay empty, got %s", ias[2].Response.Body)
	}
}
