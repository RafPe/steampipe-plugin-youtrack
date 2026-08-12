// Package integration verifies the assembled plugin across its SDK and HTTP boundaries.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RafPe/steampipe-plugin-youtrack/youtrack"
	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
)

func TestIssueListFromConnectionToHydratedRow(t *testing.T) {
	t.Parallel()

	const token = "integration-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/issues"; got != want {
			t.Errorf("issue request path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer "+token; got != want {
			t.Errorf("issue Authorization header = %q, want bearer integration token", got)
		}
		if got, want := r.URL.Query().Get("query"), "project: DEMO"; got != want {
			t.Errorf("issue query parameter = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("$top"), "1"; got != want {
			t.Errorf("issue $top parameter = %q, want %q", got, want)
		}
		if got := r.URL.Query().Get("fields"); got == "" {
			t.Error("issue fields parameter is empty, want explicit field selection")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"2-1","idReadable":"DEMO-1","summary":"integrated row","description":null,"project":{"id":"0-1","shortName":"DEMO"},"created":1786370400000}]`))
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	connection := &plugin.Connection{}
	connection.SetConfig(youtrack.Config{BaseURL: &baseURL, Token: pointer(token)})
	limit := int64(1)
	var rows []json.RawMessage
	data := &plugin.QueryData{
		Connection:   connection,
		QueryContext: &plugin.QueryContext{Limit: &limit},
		EqualsQuals: plugin.KeyColumnEqualsQualMap{
			"query": proto.NewQualValue("project: DEMO"),
		},
		StreamListItem: func(_ context.Context, items ...any) {
			encoded, err := json.Marshal(items[0])
			if err != nil {
				t.Errorf("json.Marshal(hydrated issue) error = %v, want nil", err)
				return
			}
			rows = append(rows, encoded)
		},
	}

	table := youtrack.Plugin(context.Background()).TableMap["youtrack_issue"]
	if _, err := table.List.Hydrate(context.Background(), data, nil); err != nil {
		t.Fatalf("youtrack_issue List.Hydrate() error = %v, want nil", err)
	}
	if got, want := len(rows), 1; got != want {
		t.Fatalf("youtrack_issue hydrated row count = %d, want %d", got, want)
	}

	var row map[string]any
	if err := json.Unmarshal(rows[0], &row); err != nil {
		t.Fatalf("json.Unmarshal(hydrated issue) error = %v, want nil", err)
	}
	if got, want := row["idReadable"], "DEMO-1"; got != want {
		t.Errorf("hydrated issue idReadable = %v, want %q", got, want)
	}
	project, ok := row["project"].(map[string]any)
	if !ok {
		t.Fatalf("hydrated issue project = %T, want JSON object", row["project"])
	}
	if got, want := project["id"], "0-1"; got != want {
		t.Errorf("hydrated issue project id = %v, want %q", got, want)
	}
	if got := row["description"]; got != nil {
		t.Errorf("hydrated issue description = %v, want nil", got)
	}
}

func pointer(value string) *string {
	return &value
}
