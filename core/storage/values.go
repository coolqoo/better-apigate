package storage

import (
	"encoding/json"

	"github.com/coolqoo/better-apigate/core/convention"
)

// convertValue converts a Go value to a database value.
func convertValue(val any, f convention.DerivedField) (any, error) {
	if val == nil {
		return nil, nil
	}

	switch f.Type {
	case "json", "strings", "ints":
		// JSON fields use TEXT columns. Preserve already-encoded input from
		// CLI clients and serialize structured values received over HTTP.
		switch v := val.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		default:
			encoded, err := json.Marshal(val)
			if err != nil {
				return nil, err
			}
			return string(encoded), nil
		}
	case "bool":
		switch v := val.(type) {
		case bool:
			if v {
				return 1, nil
			}
			return 0, nil
		case string:
			if v == "true" || v == "1" {
				return 1, nil
			}
			return 0, nil
		default:
			return 0, nil
		}
	case "secret", "bytes":
		// Keep binary data as []byte for BLOB storage
		// If passed as string (legacy), convert to bytes
		if s, ok := val.(string); ok {
			return []byte(s), nil
		}
		return val, nil
	default:
		return val, nil
	}
}

// convertFromDB converts a database value to a Go value.
func convertFromDB(val any, f convention.DerivedField) any {
	if val == nil {
		return nil
	}

	switch f.Type {
	case "bool":
		switch v := val.(type) {
		case int64:
			return v != 0
		case int:
			return v != 0
		default:
			return false
		}
	case "secret", "bytes":
		// Keep binary data as []byte - don't convert to string
		// This is important for bcrypt hashes and other binary data
		if b, ok := val.([]byte); ok {
			return b
		}
		// If stored as string (legacy), convert back to bytes
		if s, ok := val.(string); ok {
			return []byte(s)
		}
		return val
	default:
		// Handle byte slices as strings for text fields
		if b, ok := val.([]byte); ok {
			return string(b)
		}
		return val
	}
}
