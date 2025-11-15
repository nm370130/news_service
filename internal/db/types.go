package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringSlice is a thin wrapper around []string that implements
// sql.Scanner and driver.Valuer so it works transparently with jsonb/text columns.
type StringSlice []string

// Scan implements sql.Scanner
func (s *StringSlice) Scan(src interface{}) error {
	if s == nil {
		return fmt.Errorf("dbtypes: Scan on nil *StringSlice")
	}
	if src == nil {
		*s = []string{}
		return nil
	}

	switch v := src.(type) {
	case []byte:
		var out []string
		if err := json.Unmarshal(v, &out); err != nil {
			return err
		}
		*s = out
		return nil
	case string:
		var out []string
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return err
		}
		*s = out
		return nil
	default:
		return fmt.Errorf("dbtypes: cannot scan type %T into StringSlice", src)
	}
}

// Value implements driver.Valuer
// Marshals the slice to JSON (works well with jsonb columns).
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}