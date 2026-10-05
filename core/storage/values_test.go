package storage

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/coolqoo/better-apigate/core/convention"
	"github.com/coolqoo/better-apigate/core/schema"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestConvertValueJSONTextEncoding(t *testing.T) {
	tests := []struct {
		name  string
		field schema.FieldType
		input any
		want  string
	}{
		{"methods from HTTP", schema.FieldTypeJSON, []any{"GET", "POST"}, `["GET","POST"]`},
		{"typed methods", schema.FieldTypeJSON, []string{"GET", "POST"}, `["GET","POST"]`},
		{"all methods", schema.FieldTypeJSON, []any{}, `[]`},
		{"encoded methods", schema.FieldTypeJSON, `["GET","POST"]`, `["GET","POST"]`},
		{"encoded bytes", schema.FieldTypeJSON, []byte(`["GET","POST"]`), `["GET","POST"]`},
		{"raw JSON", schema.FieldTypeJSON, json.RawMessage(`["GET","POST"]`), `["GET","POST"]`},
		{"transform object", schema.FieldTypeJSON, map[string]any{"headers": map[string]any{"X-Test": "value"}}, `{"headers":{"X-Test":"value"}}`},
		{"string array", schema.FieldTypeStrings, []string{"GET", "POST"}, `["GET","POST"]`},
		{"integer array", schema.FieldTypeInts, []int{1, 2}, `[1,2]`},
		{"JSON scalar", schema.FieldTypeJSON, true, `true`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := convertValue(tt.input, convention.DerivedField{Type: tt.field})
			if err != nil {
				t.Fatalf("convertValue: %v", err)
			}
			if value != tt.want {
				t.Fatalf("database value = %#v, want %q", value, tt.want)
			}
			// Use pgx's TEXT codec to reproduce the reported OID 25 failure.
			encoded, err := pgtype.NewMap().Encode(pgtype.TextOID, pgtype.TextFormatCode, value, nil)
			if err != nil {
				t.Fatalf("encode PostgreSQL TEXT: %v", err)
			}
			if string(encoded) != tt.want {
				t.Errorf("stored JSON = %q, want %q", encoded, tt.want)
			}
		})
	}
}

func TestConvertValueJSONNull(t *testing.T) {
	value, err := convertValue(nil, convention.DerivedField{Type: schema.FieldTypeJSON})
	if err != nil || value != nil {
		t.Fatalf("convertValue(nil) = %#v, %v; want nil, nil", value, err)
	}
}

func TestConvertValueJSONEncodingError(t *testing.T) {
	_, err := convertValue(map[string]any{"unsupported": func() {}}, convention.DerivedField{Type: schema.FieldTypeJSON})
	var unsupported *json.UnsupportedTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v, want json.UnsupportedTypeError", err)
	}
}
