package contract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/RafPe/steampipe-plugin-youtrack/youtrack"
	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
)

// These tests are intentionally request-level contracts. They describe which
// equality predicates the plugin may push to YouTrack; predicates not listed in
// a table's KeyColumns must continue to be evaluated by Steampipe.
func TestPushdownKeyColumns(t *testing.T) {
	t.Parallel()

	wantList := map[string][]string{
		"youtrack_issue":         {"query", "project_id"},
		"youtrack_group":         {"query"},
		"youtrack_tag":           {"query"},
		"youtrack_issue_comment": {"issue_id"},
		"youtrack_issue_work_item": {
			"issue_id", "query", "start_date", "end_date", "start", "end",
			"created_start", "created_end", "updated_start", "updated_end", "author_filter", "creator_filter",
		},
	}
	wantGet := map[string][]string{
		"youtrack_project":       {"id", "short_name"},
		"youtrack_issue":         {"id", "id_readable"},
		"youtrack_user":          {"id", "login"},
		"youtrack_group":         {"id"},
		"youtrack_tag":           {"id"},
		"youtrack_article":       {"id", "id_readable"},
		"youtrack_issue_comment": {"id", "issue_id"},
	}

	for name, table := range youtrack.Plugin(context.Background()).TableMap {
		if want, ok := wantList[name]; ok {
			assertKeyNames(t, name+" list", table.List.KeyColumns, want)
		}
		if want, ok := wantGet[name]; ok {
			assertKeyNames(t, name+" get", table.Get.KeyColumns, want)
		}
	}
}

func TestOutgoingPushdownRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		table     string
		get       bool
		quals     map[string]string
		wantPath  string
		wantQuery url.Values
	}{
		{name: "issue query", table: "youtrack_issue", quals: map[string]string{"query": "project: DEMO"}, wantPath: "/api/issues", wantQuery: url.Values{"query": {"project: DEMO"}}},
		{name: "issue readable ID", table: "youtrack_issue", get: true, quals: map[string]string{"id_readable": "DEMO-7"}, wantPath: "/api/issues/DEMO-7"},
		{name: "user login is one GET", table: "youtrack_user", get: true, quals: map[string]string{"login": "alice"}, wantPath: "/api/users/alice"},
		{name: "group query", table: "youtrack_group", quals: map[string]string{"query": "team"}, wantPath: "/api/groups", wantQuery: url.Values{"query": {"team"}}},
		{name: "tag query", table: "youtrack_tag", quals: map[string]string{"query": "release"}, wantPath: "/api/tags", wantQuery: url.Values{"query": {"release"}}},
		{name: "project database ID", table: "youtrack_project", get: true, quals: map[string]string{"id": "0-3"}, wantPath: "/api/admin/projects/0-3"},
		{name: "project short name", table: "youtrack_project", get: true, quals: map[string]string{"short_name": "DEMO"}, wantPath: "/api/admin/projects/DEMO"},
		{name: "article database ID", table: "youtrack_article", get: true, quals: map[string]string{"id": "6-2"}, wantPath: "/api/articles/6-2"},
		{name: "article readable ID", table: "youtrack_article", get: true, quals: map[string]string{"id_readable": "DEMO-A-2"}, wantPath: "/api/articles/DEMO-A-2"},
		{name: "comment parent and ID", table: "youtrack_issue_comment", get: true, quals: map[string]string{"issue_id": "DEMO-7", "id": "4-9"}, wantPath: "/api/issues/DEMO-7/comments/4-9"},
		{name: "work items by parent issue", table: "youtrack_issue_work_item", quals: map[string]string{"issue_id": "DEMO-7"}, wantPath: "/api/issues/DEMO-7/timeTracking/workItems"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := invokeAndCaptureRequest(t, tt.table, tt.get, tt.quals)
			if got.URL.Path != tt.wantPath {
				t.Errorf("request path = %q, want %q", got.URL.Path, tt.wantPath)
			}
			for key, want := range tt.wantQuery {
				if values := got.URL.Query()[key]; !reflect.DeepEqual(values, want) {
					t.Errorf("query parameter %q = %v, want %v", key, values, want)
				}
			}
		})
	}
}

func TestUnsupportedPredicatesRemainLocal(t *testing.T) {
	t.Parallel()

	table := youtrack.Plugin(context.Background()).TableMap["youtrack_issue"]
	assertKeyNames(t, "issue list", table.List.KeyColumns, []string{"query", "project_id"})
	got := invokeAndCaptureRequest(t, "youtrack_issue", false, map[string]string{
		"summary": "local only",
		"query":   "project: DEMO",
	})
	if got.URL.Query().Get("summary") != "" {
		t.Errorf("unsupported summary predicate leaked into request: %s", got.URL.RawQuery)
	}
	if got.URL.Query().Get("query") != "project: DEMO" {
		t.Errorf("supported query predicate was not pushed: %s", got.URL.RawQuery)
	}
}

func invokeAndCaptureRequest(t *testing.T, tableName string, get bool, quals map[string]string) *http.Request {
	t.Helper()
	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if get {
			_, _ = w.Write([]byte(`{"id":"result"}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	token := "contract-token"
	baseURL := server.URL + "/api"
	connection := &plugin.Connection{}
	connection.SetConfig(youtrack.Config{BaseURL: &baseURL, Token: &token})
	d := &plugin.QueryData{
		Connection:     connection,
		QueryContext:   &plugin.QueryContext{},
		EqualsQuals:    make(plugin.KeyColumnEqualsQualMap),
		StreamListItem: func(context.Context, ...any) {},
	}
	for name, value := range quals {
		d.EqualsQuals[name] = proto.NewQualValue(value)
	}

	table := youtrack.Plugin(context.Background()).TableMap[tableName]
	var err error
	if get {
		_, err = table.Get.Hydrate(context.Background(), d, nil)
	} else {
		_, err = table.List.Hydrate(context.Background(), d, nil)
	}
	if err != nil {
		t.Fatalf("hydrate %s: %v", tableName, err)
	}

	select {
	case request := <-requests:
		return request
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("hydrate %s sent no request for qualifiers %v", tableName, quals)
		return nil
	}
}

func assertKeyNames(t *testing.T, label string, columns plugin.KeyColumnSlice, want []string) {
	t.Helper()
	got := make([]string, len(columns))
	for i, column := range columns {
		got[i] = column.Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s KeyColumns = %v, want %v (all other predicates must remain local)", label, got, want)
	}
}
