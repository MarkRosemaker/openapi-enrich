package cassette_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestNewResponse_LeavesBinaryBodiesAlone(t *testing.T) {
	zip := []byte("PK\x03\x04 a zip")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/zip"}},
		Body:       io.NopCloser(bytes.NewReader(zip)),
	}

	r, err := cassette.NewResponse(resp)
	if err != nil {
		t.Fatal(err)
	}

	if !r.BodyOmitted || len(r.Body) != 0 {
		t.Errorf("got omitted %v and %d bytes, want the zip omitted", r.BodyOmitted, len(r.Body))
	}

	// the caller still reads the whole zip
	if got, err := io.ReadAll(resp.Body); err != nil || !bytes.Equal(got, zip) {
		t.Errorf("the body left to read is %q, %v", got, err)
	}
}

func TestTrimBodies(t *testing.T) {
	long := strings.Repeat("ä", cassette.MaxStringLen) // two bytes each, so the cut falls inside one
	key := strings.Repeat("k", cassette.MaxStringLen+1)

	ias := cassette.Interactions{{
		Request: cassette.Request{Method: http.MethodGet, URL: url.URL{Scheme: "https", Host: "api.example.com"}},
		Response: cassette.Response{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": {"application/json"}},
			Body:       cassette.Body(`{"image":"` + long + `","` + key + `":"short","list":["` + long + `"]}`),
		},
	}, {
		Request: cassette.Request{Method: http.MethodGet, URL: url.URL{Scheme: "https", Host: "api.example.com", Path: "/report"}},
		Response: cassette.Response{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": {"application/pdf"}},
			Body:       cassette.Body("%PDF-1.7 ..."),
		},
	}}

	ias.TrimBodies(cassette.MaxStringLen)

	body := string(ias[0].Response.Body)
	cut := strings.Repeat("ä", cassette.MaxStringLen/2) + "…"

	if want := `{"image":"` + cut + `","` + key + `":"short","list":["` + cut + `"]}`; body != want {
		t.Errorf("got a body of %d bytes, want long strings cut and member names kept", len(body))
	}

	if pdf := ias[1].Response; !pdf.BodyOmitted || len(pdf.Body) != 0 {
		t.Errorf("the PDF is still recorded: omitted %v, %d bytes", pdf.BodyOmitted, len(pdf.Body))
	}
}
