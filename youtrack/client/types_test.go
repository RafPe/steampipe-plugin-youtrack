package client

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTimestampUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		want time.Time
	}{
		{name: "milliseconds", json: `1720000000123`, want: time.UnixMilli(1720000000123).UTC()},
		{name: "RFC3339", json: `"2024-07-03T09:46:40.123Z"`, want: time.Date(2024, 7, 3, 9, 46, 40, 123000000, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got Timestamp
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("json.Unmarshal(%s) error = %v, want nil", tt.json, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("json.Unmarshal(%s) = %v, want %v", tt.json, got.Time, tt.want)
			}
		})
	}
}

func TestTimestampNullErrorsAndMarshal(t *testing.T) {
	var timestamp Timestamp
	if err := json.Unmarshal([]byte("null"), &timestamp); err != nil || !timestamp.IsZero() {
		t.Fatalf("unmarshal null = %v, %v", timestamp.Time, err)
	}
	for _, input := range []string{"{}", `"not-a-time"`} {
		if err := json.Unmarshal([]byte(input), &timestamp); err == nil {
			t.Fatalf("json.Unmarshal(%s) error = nil", input)
		}
	}
	data, err := json.Marshal(Timestamp{})
	if err != nil || string(data) != "null" {
		t.Fatalf("marshal zero timestamp = %s, %v", data, err)
	}
	timestamp = Timestamp{Time: time.UnixMilli(1720000000123)}
	data, err = json.Marshal(timestamp)
	if err != nil || string(data) != "1720000000123" {
		t.Fatalf("marshal timestamp = %s, %v", data, err)
	}
}

func TestJSONValueErrorsAndMarshal(t *testing.T) {
	var value JSONValue
	if err := value.UnmarshalJSON([]byte("{")); err == nil || !strings.Contains(err.Error(), "decode polymorphic YouTrack value") {
		t.Fatalf("unmarshal malformed value = %v", err)
	}
	data, err := json.Marshal(JSONValue{Value: json.Number("42")})
	if err != nil || string(data) != "42" {
		t.Fatalf("marshal JSONValue = %s, %v", data, err)
	}
	_, err = json.Marshal(JSONValue{Value: func() {}})
	if err == nil {
		t.Fatal("marshal unsupported JSONValue error = nil")
	}
}

func TestSanitizedFixtureDecodesTimestampAndPolymorphicValues(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/issue.json")
	if err != nil {
		t.Fatalf("os.ReadFile(testdata/issue.json) error = %v, want nil", err)
	}
	var got struct {
		Created      Timestamp `json:"created"`
		CustomFields []struct {
			Value JSONValue `json:"value"`
		} `json:"customFields"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(testdata/issue.json) error = %v, want nil", err)
	}
	if want := time.UnixMilli(1720000000123); !got.Created.Equal(want) {
		t.Errorf("fixture created = %v, want %v", got.Created.Time, want)
	}
	if gotLen := len(got.CustomFields); gotLen != 2 {
		t.Fatalf("len(fixture customFields) = %d, want 2", gotLen)
	}
	if _, ok := got.CustomFields[0].Value.Value.(map[string]any); !ok {
		t.Errorf("fixture State value type = %T, want map[string]any", got.CustomFields[0].Value.Value)
	}
	if _, ok := got.CustomFields[1].Value.Value.([]any); !ok {
		t.Errorf("fixture Tags value type = %T, want []any", got.CustomFields[1].Value.Value)
	}
}

func TestJSONValueSupportsPolymorphicValues(t *testing.T) {
	t.Parallel()

	input := `[{"name":"State"},{"name":"Tags"},"plain",42,true,null]`
	var got JSONValue
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v, want nil", input, err)
	}
	want := []any{map[string]any{"name": "State"}, map[string]any{"name": "Tags"}, "plain", json.Number("42"), true, nil}
	if !reflect.DeepEqual(got.Value, want) {
		t.Errorf("JSONValue = %#v, want %#v", got.Value, want)
	}
}
