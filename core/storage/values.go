package storage

import "github.com/coolqoo/better-apigate/core/convention"

// convertValue converts a Go value to a database value.
func convertValue(val any, f convention.DerivedField) any {
	if val == nil {
		return nil
	}

	switch f.Type {
	case "bool":
		switch v := val.(type) {
		case bool:
			if v {
				return 1
			}
			return 0
		case string:
			if v == "true" || v == "1" {
				return 1
			}
			return 0
		default:
			return 0
		}
	case "secret", "bytes":
		// Keep binary data as []byte for BLOB storage
		// If passed as string (legacy), convert to bytes
		if s, ok := val.(string); ok {
			return []byte(s)
		}
		return val
	default:
		return val
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
