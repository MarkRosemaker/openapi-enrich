package enrich

import (
	"bytes"
	"encoding/json/jsontext"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
	"uuid"

	"github.com/MarkRosemaker/openapi"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
	merge "github.com/MarkRosemaker/openapi-merge"
	apitypes "github.com/go-api-libs/types"
)

// newSchemaFromJSON infers an OpenAPI schema from a JSON-encoded value.
func newSchemaFromJSON(data []byte) (*openapi.Schema, error) {
	return decodeSchema(jsontext.NewDecoder(bytes.NewReader(data)))
}

func decodeSchema(dec *jsontext.Decoder) (*openapi.Schema, error) {
	v, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}

	switch v.Kind() {
	case '"': // string
		s := &openapi.Schema{
			Type:   openapi.TypeString,
			Format: stringFormat(v.String()),
		}

		// A redacted value makes a poor example: it says nothing the inferred
		// format does not already say, and reads as though the API returns
		// asterisks. Masking is shape-preserving, so the format survives without
		// it.
		if !cassette.IsMasked(v.String()) {
			s.Example = jsontext.Value(fmt.Appendf(nil, "%q", v.String()))
		}

		return s, nil

	case '0': // number
		str := v.String()
		if _, err := strconv.Atoi(str); err == nil {
			return &openapi.Schema{Type: openapi.TypeInteger, Example: jsontext.Value(str)}, nil
		}

		return &openapi.Schema{Type: openapi.TypeNumber, Format: openapi.FormatDouble, Example: jsontext.Value(str)}, nil

	case 't': // true
		return &openapi.Schema{Type: openapi.TypeBoolean}, nil
	case 'f': // false
		return &openapi.Schema{Type: openapi.TypeBoolean}, nil
	case 'n': // null → object placeholder; isGeneratedFromNull in openapi-merge checks Example=="null"
		// TODO: perhaps it would be better to actually set TypeNull here
		return &openapi.Schema{Type: openapi.TypeObject, Example: jsontext.Value("null")}, nil
	case '{': // begin object
		return decodeObjectSchema(dec)
	case '[': // begin array
		return decodeArraySchema(dec)
	default:
		return nil, fmt.Errorf("unexpected token type %s", v.Kind())
	}
}

func decodeObjectSchema(dec *jsontext.Decoder) (*openapi.Schema, error) {
	type kv struct {
		key    string
		schema *openapi.Schema
	}

	var pairs []kv

	for dec.PeekKind() != '}' {
		keyTok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}

		if keyTok.Kind() != '"' {
			return nil, fmt.Errorf("expected string key, got %s", keyTok)
		}

		key := keyTok.String()

		propSchema, err := decodeSchema(dec)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", key, err)
		}

		// A redacted number is indistinguishable from a real one by value, so
		// drop the example by key instead. These keys hold credentials, which
		// have no business appearing as examples either way.
		if cassette.RedactsBodyKey(key) {
			propSchema.Example = nil
		}

		pairs = append(pairs, kv{key, propSchema})
	}

	if _, err := dec.ReadToken(); err != nil { // consume '}'
		return nil, err
	}

	if len(pairs) == 0 {
		return &openapi.Schema{Type: openapi.TypeObject}, nil
	}

	// If every key is a stringified non-negative integer the object is a
	// numeric-keyed map (e.g. {"0":{...},"1":{...}}). Model it with
	// additionalProperties so the key pattern is explicit and the schema stays
	// compact regardless of how many entries are present.
	allNumeric := true
	for _, p := range pairs {
		if !isNumericKey(p.key) {
			allNumeric = false
			break
		}
	}

	if allNumeric {
		var valueSchema *openapi.Schema
		for _, p := range pairs {
			if valueSchema == nil {
				valueSchema = p.schema
				continue
			}

			if err := merge.Schema(valueSchema, p.schema, false); err != nil {
				return nil, fmt.Errorf("merging additionalProperties value: %w", err)
			}
		}

		return &openapi.Schema{
			Type:                 openapi.TypeObject,
			AdditionalProperties: &openapi.SchemaRef{Value: valueSchema},
		}, nil
	}

	// Named properties — build the schema as before.
	s := &openapi.Schema{
		Type:       openapi.TypeObject,
		Properties: openapi.SchemaRefs{},
	}
	for _, p := range pairs {
		s.Properties.Set(p.key, &openapi.SchemaRef{Value: p.schema})
		s.Required = append(s.Required, p.key)
	}

	return s, nil
}

// isNumericKey reports whether s consists entirely of ASCII digits.
func isNumericKey(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

func decodeArraySchema(dec *jsontext.Decoder) (*openapi.Schema, error) {
	s := &openapi.Schema{Type: openapi.TypeArray}

	var groups []*openapi.Schema
	for dec.PeekKind() != ']' {
		elem, err := decodeSchema(dec)
		if err != nil {
			return nil, err
		}

		groups = mergeIntoGroups(groups, elem)
	}

	if _, err := dec.ReadToken(); err != nil { // consume ']'
		return nil, err
	}

	s.Items = &openapi.SchemaRef{Value: itemsSchemaFromGroups(groups)}

	return s, nil
}

// mergeIntoGroups merges elem into the first of groups it can be merged
// with, or appends it as a new group when it can be merged with none of
// them: a fixed-size, positionally-typed array (e.g. OpenSky's state
// vectors: [icao24 string, ..., time_position int, ..., on_ground bool,
// ...]) mixes types no single merged item schema can represent.
func mergeIntoGroups(groups []*openapi.Schema, elem *openapi.Schema) []*openapi.Schema {
	for _, g := range groups {
		if err := merge.Schema(g, elem, false); err == nil {
			return groups
		}
	}

	return append(groups, elem)
}

// itemsSchemaFromGroups turns the distinct type groups found in an array
// into its items schema: the group itself when there is only one, or --
// for a tuple-shaped array -- a oneOf of every group observed, since this
// package has no positional (prefixItems) schema to fall back on.
func itemsSchemaFromGroups(groups []*openapi.Schema) *openapi.Schema {
	switch len(groups) {
	case 0:
		// empty array → placeholder object items, refined on non-empty array
		return &openapi.Schema{Type: openapi.TypeObject, Example: jsontext.Value("null")}
	case 1:
		return groups[0]
	default:
		alternatives := make(openapi.SchemaRefList, len(groups))
		for i, g := range groups {
			alternatives[i] = &openapi.SchemaRef{Value: g}
		}

		return &openapi.Schema{OneOf: alternatives}
	}
}

// stringFormat detects the special format for a string value.
// It tries in order: UUID, URI, Email, DateTime (RFC3339), IPv4/IPv6.
func stringFormat(s string) openapi.Format {
	if isUUID(s) {
		return openapi.FormatUUID
	}

	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" {
		return openapi.FormatURI
	}

	if apitypes.Email(s).Validate() == nil {
		return openapi.FormatEmail
	}

	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return openapi.FormatDateTime
	}

	if ip := net.ParseIP(s); ip != nil {
		if ip.To4() != nil {
			return openapi.FormatIPv4
		}

		return openapi.FormatIPv6
	}

	return ""
}

// isUUID reports whether s matches the UUID format.
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
