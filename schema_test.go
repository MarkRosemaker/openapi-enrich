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
