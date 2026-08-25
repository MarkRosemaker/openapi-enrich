package cassette_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MarkRosemaker/openapi-enrich/cassette"
)

func TestMask_Headers(t *testing.T) {
	ias := cassette.Interactions{{
		Request: cassette.Request{Headers: http.Header{
			"Authorization": {"Bearer secret-token-value"},
			"X-Api-User":    {"11111111-2222-3333-4444-555555555555"},
			"Accept":        {"application/json"},
		}},
		Response: cassette.Response{Headers: http.Header{
			"Set-Cookie":   {"sess=0.11111111.2222222222.abcdef0; secure"},
			"Content-Type": {"application/json"},
		}},
	}}

	ias.Mask()

	req, resp := ias[0].Request.Headers, ias[0].Response.Headers

	if got := req.Get("Authorization"); got != "Bearer "+strings.Repeat("*", len("secret-token-value")) {
		t.Errorf("Authorization = %q", got)
	}
	// Shape-preserving: a UUID is replaced by a UUID.
	if got := req.Get("X-Api-User"); got != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("X-Api-User = %q", got)
	}
	if got := req.Get("Accept"); got != "application/json" {
		t.Errorf("Accept should be untouched, got %q", got)
	}
	// Response headers are masked too, not just request headers.
	if got := resp.Get("Set-Cookie"); got == "sess=0.11111111.2222222222.abcdef0; secure" {
		t.Error("Set-Cookie was not masked")
	}
	if got := resp.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type should be untouched, got %q", got)
	}
}

func TestMask_Body(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{{
		name: "matching key is masked, others are not",
		in:   `{"session":"abcdef0123456789abcdef0123456789","statusCode":200}`,
		want: `{"session":"00000000000000000000000000000000","statusCode":200}`,
	}, {
		name: "a masked number stays a number",
		in:   `{"userId":12345678}`,
		want: `{"userId":0}`,
	}, {
		name: "matching a key redacts the whole subtree",
		in:   `{"credentials":{"user":"me","nested":{"n":7}},"keep":"me"}`,
		want: `{"credentials":{"user":"**","nested":{"n":0}},"keep":"me"}`,
	}, {
		name: "nested and repeated keys are all masked",
		in:   `{"a":{"uuid":"99999999-8888-7777-6666-555555555555"},"b":[{"uuid":"99999999-8888-7777-6666-555555555555"}]}`,
		want: `{"a":{"uuid":"00000000-0000-0000-0000-000000000000"},"b":[{"uuid":"00000000-0000-0000-0000-000000000000"}]}`,
	}, {
		name: "key order is preserved",
		in:   `{"z":1,"m":2,"a":3}`,
		want: `{"z":1,"m":2,"a":3}`,
	}, {
		name: "matching is case-insensitive",
		in:   `{"MAC":"de:ad:be:ef:00:01"}`,
		want: `{"MAC":"00:00:00:00:00:00"}`,
	}, {
		name: "unrelated data is left alone",
		in:   `{"items":[1,2,3],"ok":true,"name":null}`,
		want: `{"items":[1,2,3],"ok":true,"name":null}`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ias := cassette.Interactions{{
				Response: cassette.Response{Body: cassette.Body(tc.in)},
			}}

			ias.Mask()

			if got := string(ias[0].Response.Body); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestMask_Values covers identifiers living under keys too generic to redact
// wholesale: masking every "id" would destroy unrelated data, so the value
// itself is listed instead.
func TestMask_Values(t *testing.T) {
	const accountID = "11111111-2222-3333-4444-555555555555"

	ias := cassette.Interactions{{
		Response: cassette.Response{Body: cassette.Body(
			`{"id":"` + accountID + `","ownerId":"` + accountID + `","other":"keep-me","n":5}`,
		)},
	}}

	m := cassette.DefaultMasker()
	m.Values = []string{accountID}
	ias.MaskWith(m)

	want := `{"id":"00000000-0000-0000-0000-000000000000",` +
		`"ownerId":"00000000-0000-0000-0000-000000000000","other":"keep-me","n":5}`
	if got := string(ias[0].Response.Body); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestMask_ValueInsideLargerString covers an identifier embedded in a longer
// string, such as a user agent or a URL, rather than being the whole value.
func TestMask_ValueInsideLargerString(t *testing.T) {
	ias := cassette.Interactions{{
		Response: cassette.Response{Body: cassette.Body(
			`{"note":"contact someone@personal.test for access"}`,
		)},
	}}

	m := cassette.DefaultMasker()
	m.Values = []string{"someone@personal.test"}
	ias.MaskWith(m)

	want := `{"note":"contact john.doe@example.com for access"}`
	if got := string(ias[0].Response.Body); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestMask_InvalidJSONUntouched: masking must never corrupt a recording.
func TestMask_InvalidJSONUntouched(t *testing.T) {
	const body = `<html>not json at all</html>`

	ias := cassette.Interactions{{
		Response: cassette.Response{Body: cassette.Body(body)},
	}}

	ias.Mask()

	if got := string(ias[0].Response.Body); got != body {
		t.Errorf("non-JSON body was altered: %q", got)
	}
}

func TestMask_EmptyBody(t *testing.T) {
	ias := cassette.Interactions{{Response: cassette.Response{}}}

	ias.Mask() // must not panic

	if len(ias[0].Response.Body) != 0 {
		t.Errorf("empty body became %q", ias[0].Response.Body)
	}
}

// TestMask_IDKeys covers keys too generic to redact outright: an account
// identifier under "id" must go, while a structural name under the same key
// must stay.
func TestMask_IDKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{{
		name: "a structural name is left alone",
		in:   `{"id":"title"}`,
		want: `{"id":"title"}`,
	}, {
		name: "a UUID is redacted",
		in:   `{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}`,
		want: `{"id":"00000000-0000-0000-0000-000000000000"}`,
	}, {
		name: "a long numeric account id is redacted",
		in:   `{"id":"100200300400500600700"}`,
		want: `{"id":"*********************"}`,
	}, {
		name: "prose of the same length is left alone",
		in:   `{"id":"a rather long descriptive name"}`,
		want: `{"id":"a rather long descriptive name"}`,
	}, {
		name: "a small numeric id keeps its value",
		in:   `{"id":7}`,
		want: `{"id":7}`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ias := cassette.Interactions{{
				Response: cassette.Response{Body: cassette.Body(tc.in)},
			}}

			m := cassette.DefaultMasker()
			m.IDKeys = []string{"id"}
			ias.MaskWith(m)

			if got := string(ias[0].Response.Body); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestIsMasked(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"00000000-0000-0000-0000-000000000000", true},
		{"00000000000000000000000000000000", true},
		{"john.doe@example.com", true},
		{"00:00:00:00:00:00", true},
		{"*****", true},
		{"Bearer ******", true},
		{"", false},
		{"title", false},
		{"someone@personal.test", false},
		{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", false},
		{"5 * 3 = 15", false},
	} {
		if got := cassette.IsMasked(tc.in); got != tc.want {
			t.Errorf("IsMasked(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestMask_NameKeys covers replacing a person's name, and the shape check that
// keeps capitalised phrases which are not people intact.
func TestMask_NameKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{{
		name: "a person's name is replaced",
		in:   `{"user":"Frieda Muster"}`,
		want: `{"user":"John Doe"}`,
	}, {
		name: "a three-part name is replaced",
		in:   `{"user":"Ada King Lovelace"}`,
		want: `{"user":"John Doe"}`,
	}, {
		name: "a hyphenated name is replaced",
		in:   `{"user":"Marie-Claire Dubois"}`,
		want: `{"user":"John Doe"}`,
	}, {
		name: "a company name is left alone",
		in:   `{"user":"Free Public APIs API"}`,
		want: `{"user":"Free Public APIs API"}`,
	}, {
		name: "a single word is left alone",
		in:   `{"user":"anonymous"}`,
		want: `{"user":"anonymous"}`,
	}, {
		name: "a handle is left alone",
		in:   `{"user":"user_12345"}`,
		want: `{"user":"user_12345"}`,
	}, {
		name: "a name under an unlisted key is left alone",
		in:   `{"breed":"Domestic Shorthair"}`,
		want: `{"breed":"Domestic Shorthair"}`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ias := cassette.Interactions{{
				Response: cassette.Response{Body: cassette.Body(tc.in)},
			}}

			m := cassette.DefaultMasker()
			m.NameKeys = []string{"user"}
			ias.MaskWith(m)

			if got := string(ias[0].Response.Body); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestRedactsBodyKey(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"userId", true},
		{"USERID", true}, // case-insensitive
		{"session", true},
		{"password", true},
		{"id", false}, // too generic for the default masker
		{"count", false},
		{"", false},
	} {
		if got := cassette.RedactsBodyKey(tc.in); got != tc.want {
			t.Errorf("RedactsBodyKey(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestMask_UsernameKeys covers replacing a handle while keeping its case, so
// that an API storing the same handle several ways still looks consistent.
func TestMask_UsernameKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{{
		name: "lower case stays lower case",
		in:   `{"username":"frieda"}`,
		want: `{"username":"johndoe"}`,
	}, {
		name: "capitalised stays capitalised",
		in:   `{"username":"FriedaMuster"}`,
		want: `{"username":"JohnDoe"}`,
	}, {
		name: "upper case stays upper case",
		in:   `{"username":"FRIEDA"}`,
		want: `{"username":"JOHNDOE"}`,
	}, {
		name: "a leading @ is kept",
		in:   `{"username":"@frieda"}`,
		want: `{"username":"@johndoe"}`,
	}, {
		name: "a leading @ with capitals is kept",
		in:   `{"username":"@FriedaMuster"}`,
		want: `{"username":"@JohnDoe"}`,
	}, {
		name: "both spellings of one handle stay consistent",
		in:   `{"username":"FriedaMuster","lowerCaseUsername":"friedamuster"}`,
		want: `{"username":"JohnDoe","lowerCaseUsername":"johndoe"}`,
	}, {
		name: "an empty handle is left alone",
		in:   `{"username":""}`,
		want: `{"username":""}`,
	}, {
		name: "an unlisted key is left alone",
		in:   `{"nickname":"frieda"}`,
		want: `{"nickname":"frieda"}`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ias := cassette.Interactions{{
				Response: cassette.Response{Body: cassette.Body(tc.in)},
			}}

			m := cassette.DefaultMasker()
			m.UsernameKeys = []string{"username", "lowerCaseUsername"}
			ias.MaskWith(m)

			if got := string(ias[0].Response.Body); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestIsMasked_Personas(t *testing.T) {
	for _, s := range []string{
		"john.doe@example.com", "John Doe",
		"johndoe", "JohnDoe", "JOHNDOE", "@johndoe", "@JohnDoe",
	} {
		if !cassette.IsMasked(s) {
			t.Errorf("IsMasked(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"john", "doe", "John Doe Smith Jones Brown"} {
		if cassette.IsMasked(s) {
			t.Errorf("IsMasked(%q) = true, want false", s)
		}
	}
}

func TestMask_RdToken(t *testing.T) {
	// X-Rd-Token is an API token. The non-canonical spelling is deliberate:
	// the masker lowercases both sides, so it matches regardless of case.
	ias := cassette.Interactions{{
		Request: cassette.Request{Headers: http.Header{
			"X-RD-Token": {"rd_live_9f8e7d6c5b4a3210"},
			"Accept":     {"application/json"},
		}},
	}}

	ias.Mask()

	got := ias[0].Request.Headers["X-RD-Token"][0] //nolint:staticcheck
	if got == "rd_live_9f8e7d6c5b4a3210" {
		t.Fatal("X-Rd-Token was not masked")
	}
	if !cassette.IsMasked(got) {
		t.Errorf("X-Rd-Token = %q, want a masked value", got)
	}
	if got := ias[0].Request.Headers.Get("Accept"); got != "application/json" {
		t.Errorf("Accept should be untouched, got %q", got)
	}
}
