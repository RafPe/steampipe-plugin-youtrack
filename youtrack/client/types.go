package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Timestamp represents a YouTrack timestamp. It accepts Unix milliseconds or
// an RFC 3339 string and emits Unix milliseconds.
type Timestamp struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		t.Time = time.Time{}
		return nil
	}
	var milliseconds int64
	if err := json.Unmarshal(data, &milliseconds); err == nil {
		t.Time = time.UnixMilli(milliseconds).UTC()
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode YouTrack timestamp: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return fmt.Errorf("parse YouTrack timestamp: %w", err)
	}
	t.Time = parsed
	return nil
}

// MarshalJSON implements json.Marshaler.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.UnixMilli())
}

// JSONValue holds a polymorphic JSON scalar, object, or array. JSON numbers
// are retained as json.Number values to avoid precision loss.
type JSONValue struct {
	Value any
}

// UnmarshalJSON implements json.Unmarshaler.
func (v *JSONValue) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&v.Value); err != nil {
		return fmt.Errorf("decode polymorphic YouTrack value: %w", err)
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (v JSONValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Value)
}
