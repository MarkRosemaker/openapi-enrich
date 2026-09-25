package enrich

import (
	"testing"

	"github.com/MarkRosemaker/openapi"
)

func TestNewSchemaFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantTyp openapi.DataType
		wantFmt openapi.Format
	}{
		// Scalars
		{"string plain", `"hello"`, openapi.TypeString, ""},
		{"string uuid", `"550e8400-e29b-41d4-a716-446655440000"`, openapi.TypeString, openapi.FormatUUID},
		{"string uri", `"https://example.com/path"`, openapi.TypeString, openapi.FormatURI},
		{"string email", `"user@example.com"`, openapi.TypeString, openapi.FormatEmail},
		{"string datetime", `"2024-01-15T10:30:00Z"`, openapi.TypeString, openapi.FormatDateTime},
		{"string ipv4", `"192.168.1.1"`, openapi.TypeString, openapi.FormatIPv4},
		{"string ipv6", `"2001:db8::1"`, openapi.TypeString, openapi.FormatIPv6},
		{"integer", `42`, openapi.TypeInteger, ""},
		{"negative integer", `-7`, openapi.TypeInteger, ""},
		{"float number", `3.14`, openapi.TypeNumber, openapi.FormatDouble},
		{"bool true", `true`, openapi.TypeBoolean, ""},
		{"bool false", `false`, openapi.TypeBoolean, ""},
		// null → object placeholder
		{"null", `null`, openapi.TypeObject, ""},
		// Composite
		{"empty object", `{}`, openapi.TypeObject, ""},
		{"empty array", `[]`, openapi.TypeArray, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newSchemaFromJSON([]byte(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if s.Type != tc.wantTyp {
				t.Errorf("type: got %q, want %q", s.Type, tc.wantTyp)
			}

			if s.Format != tc.wantFmt {
				t.Errorf("format: got %q, want %q", s.Format, tc.wantFmt)
			}
		})
	}
}

func TestNewSchemaFromJSON_Object(t *testing.T) {
	s, err := newSchemaFromJSON([]byte(`{"id":1,"name":"Alice","active":true}`))
	if err != nil {
		t.Fatal(err)
	}

	if s.Type != openapi.TypeObject {
		t.Fatalf("type: got %q, want object", s.Type)
	}

	if len(s.Properties) != 3 {
		t.Fatalf("properties: got %d, want 3", len(s.Properties))
	}

	if len(s.Required) != 3 {
		t.Fatalf("required: got %d, want 3", len(s.Required))
	}

	if s.Properties["id"].Value.Type != openapi.TypeInteger {
		t.Errorf("id type: got %q, want integer", s.Properties["id"].Value.Type)
	}

	if s.Properties["name"].Value.Type != openapi.TypeString {
		t.Errorf("name type: got %q, want string", s.Properties["name"].Value.Type)
	}

	if s.Properties["active"].Value.Type != openapi.TypeBoolean {
		t.Errorf("active type: got %q, want boolean", s.Properties["active"].Value.Type)
	}
}

func TestNewSchemaFromJSON_Array(t *testing.T) {
	s, err := newSchemaFromJSON([]byte(`[1,2,3]`))
	if err != nil {
		t.Fatal(err)
	}

	if s.Type != openapi.TypeArray {
		t.Fatalf("type: got %q, want array", s.Type)
	}

	if s.Items == nil {
		t.Fatal("items is nil")
	}

	if s.Items.Value.Type != openapi.TypeInteger {
		t.Errorf("items type: got %q, want integer", s.Items.Value.Type)
	}
}

func TestNewSchemaFromJSON_EmptyArray(t *testing.T) {
	s, err := newSchemaFromJSON([]byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}

	if s.Type != openapi.TypeArray {
		t.Fatalf("type: got %q, want array", s.Type)
	}

	if s.Items == nil {
		t.Fatal("items is nil for empty array")
	}

	if s.Items.Value.Type != openapi.TypeObject {
		t.Errorf("items type for empty array: got %q, want object", s.Items.Value.Type)
	}
}

// TestNewSchemaFromJSON_TupleArray covers a fixed-size, positionally-typed
// array (e.g. OpenSky Network's state vectors: [icao24 string, callsign
// string, ..., time_position int, ..., on_ground bool, ...]): merging every
// element into one item schema fails outright on the first type mismatch, so
// mismatched elements must fall back to prefixItems -- one schema per
// position, exactly as this one occurrence showed it -- instead.
func TestNewSchemaFromJSON_TupleArray(t *testing.T) {
	s, err := newSchemaFromJSON([]byte(`["39de4f", 1790341107, 48.7239, true]`))
	if err != nil {
		t.Fatal(err)
	}

	if s.Type != openapi.TypeArray {
		t.Fatalf("type: got %q, want array", s.Type)
	}

	if s.Items != nil {
		t.Errorf("items: got %+v, want unset (a tuple)", s.Items)
	}

	wantTypes := []openapi.DataType{openapi.TypeString, openapi.TypeInteger, openapi.TypeNumber, openapi.TypeBoolean}
	if len(s.PrefixItems) != len(wantTypes) {
		t.Fatalf("prefixItems: got %d entries, want %d", len(s.PrefixItems), len(wantTypes))
	}

	for i, want := range wantTypes {
		if got := s.PrefixItems[i].Value.Type; got != want {
			t.Errorf("prefixItems[%d] type: got %q, want %q", i, got, want)
		}
	}
}

// TestNewSchemaFromJSON_TupleArray_NoPartialMutation covers a tuple whose
// first two elements merge successfully with each other (integer widening to
// number) before the third breaks the merge attempt outright: the abandoned
// attempt must not leave its partial work behind on the first position, since
// prefixItems then falls back to every element exactly as it was decoded.
func TestNewSchemaFromJSON_TupleArray_NoPartialMutation(t *testing.T) {
	s, err := newSchemaFromJSON([]byte(`[5, 5.5, "x"]`))
	if err != nil {
		t.Fatal(err)
	}

	if len(s.PrefixItems) != 3 {
		t.Fatalf("prefixItems: got %d entries, want 3", len(s.PrefixItems))
	}

	// position 0 must still be the plain integer it was decoded as, not the
	// number it would have widened to had the merge attempt's mutation of it
	// leaked out of the abandoned attempt.
	if got, want := s.PrefixItems[0].Value.Type, openapi.TypeInteger; got != want {
		t.Errorf("prefixItems[0] type: got %q, want %q", got, want)
	}

	if got, want := s.PrefixItems[1].Value.Type, openapi.TypeNumber; got != want {
		t.Errorf("prefixItems[1] type: got %q, want %q", got, want)
	}

	if got, want := s.PrefixItems[2].Value.Type, openapi.TypeString; got != want {
		t.Errorf("prefixItems[2] type: got %q, want %q", got, want)
	}
}

func TestNewSchemaFromJSON_NumericKeyObject(t *testing.T) {
	// Objects whose keys are all stringified integers should be inferred as
	// additionalProperties maps, not explicit properties.
	s, err := newSchemaFromJSON([]byte(`{"0":{"ticker":"NVDA","cik":1045810},"1":{"ticker":"AAPL","cik":320193}}`))
	if err != nil {
		t.Fatal(err)
	}

	if s.Type != openapi.TypeObject {
		t.Fatalf("type: got %q, want object", s.Type)
	}

	if s.AdditionalProperties == nil {
		t.Fatal("expected additionalProperties to be set for numeric-keyed object")
	}

	if s.Properties != nil {
		t.Error("expected no explicit properties for numeric-keyed object")
	}

	if s.Required != nil {
		t.Error("expected no required for numeric-keyed object")
	}
	// The value schema should be the merged entry schema.
	v := s.AdditionalProperties.Value
	if v.Type != openapi.TypeObject {
		t.Errorf("additionalProperties type: got %q, want object", v.Type)
	}

	if v.Properties["ticker"] == nil || v.Properties["ticker"].Value.Type != openapi.TypeString {
		t.Error("expected ticker:string in additionalProperties value schema")
	}
}

func TestIsNumericKey(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"0", true},
		{"42", true},
		{"007", true},
		{"", false},
		{"1a", false},
		{"id", false},
		{"-1", false},
	}
	for _, tc := range cases {
		if got := isNumericKey(tc.in); got != tc.want {
			t.Errorf("isNumericKey(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewSchemaFromJSON_NullExample(t *testing.T) {
	s, err := newSchemaFromJSON([]byte(`null`))
	if err != nil {
		t.Fatal(err)
	}

	if string(s.Example) != "null" {
		t.Errorf("null schema example: got %q, want \"null\"", string(s.Example))
	}
}

func TestStringFormat(t *testing.T) {
	tests := []struct {
		input string
		want  openapi.Format
	}{
		{"550e8400-e29b-41d4-a716-446655440000", openapi.FormatUUID},
		{"https://example.com", openapi.FormatURI},
		{"http://api.example.org/v1", openapi.FormatURI},
		{"user@example.com", openapi.FormatEmail},
		{"2024-01-15T10:30:00Z", openapi.FormatDateTime},
		{"192.168.0.1", openapi.FormatIPv4},
		{"::1", openapi.FormatIPv6},
		{"hello world", ""},
		{"just-a-string", ""},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := stringFormat(tc.input)
			if got != tc.want {
				t.Errorf("stringFormat(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
