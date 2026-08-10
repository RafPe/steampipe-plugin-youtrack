package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin/transform"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestResourcePreservesRawJSON(t *testing.T) {
	t.Parallel()

	input := []byte(`{"id":"2-1","summary":null,"description":null,"customFields":[{"value":{"name":"Open"}}]}`)
	var got resource
	if err := json.Unmarshal(input, &got); err != nil {
		t.Fatalf("json.Unmarshal(resource) error = %v, want nil", err)
	}
	if got.ID != "2-1" || got.Description != nil {
		t.Errorf("json.Unmarshal(resource) = %#v, want id 2-1 and nil description", got)
	}
	if got.Summary != nil {
		t.Errorf("json.Unmarshal(resource).Summary = %v, want nil", got.Summary)
	}
	if string(got.Raw) != string(input) {
		t.Errorf("resource.Raw = %s, want %s", got.Raw, input)
	}
	if err := json.Unmarshal([]byte(`{`), &got); err == nil {
		t.Error("json.Unmarshal(malformed resource) error = nil, want error")
	}
	if err := got.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("resource.UnmarshalJSON(malformed) error = nil, want error")
	}
}

func TestResourceDecodesExpandedTableFields(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"email":null,
		"banned":true,
		"online":false,
		"untagOnResolve":true,
		"query":"project: DEMO",
		"leader":{"id":"1-1"},
		"users":[{"id":"1-2"}],
		"projects":[{"id":"0-1"}],
		"sprints":[{"id":"3-1"}],
		"currentSprint":{"id":"3-1"},
		"readSharingSettings":{"permittedGroups":[]},
		"tagSharingSettings":{"permittedGroups":[]},
		"updateSharingSettings":{"permittedGroups":[]}
	}`)
	var got resource
	if err := json.Unmarshal(input, &got); err != nil {
		t.Fatalf("json.Unmarshal(expanded resource) error = %v, want nil", err)
	}
	if got.Email != nil {
		t.Errorf("json.Unmarshal(expanded resource).Email = %v, want nil", got.Email)
	}
	if got.Banned == nil || !*got.Banned {
		t.Errorf("json.Unmarshal(expanded resource).Banned = %v, want true", got.Banned)
	}
	if got.Online == nil || *got.Online {
		t.Errorf("json.Unmarshal(expanded resource).Online = %v, want false", got.Online)
	}
	if got.UntagOnResolve == nil || !*got.UntagOnResolve {
		t.Errorf("json.Unmarshal(expanded resource).UntagOnResolve = %v, want true", got.UntagOnResolve)
	}
	if got.Query != "project: DEMO" {
		t.Errorf("json.Unmarshal(expanded resource).Query = %q, want %q", got.Query, "project: DEMO")
	}
	for name, value := range map[string]json.RawMessage{
		"Leader": got.Leader, "Users": got.Users, "Projects": got.Projects,
		"Sprints": got.Sprints, "CurrentSprint": got.CurrentSprint,
		"ReadSharingSettings":   got.ReadSharingSettings,
		"TagSharingSettings":    got.TagSharingSettings,
		"UpdateSharingSettings": got.UpdateSharingSettings,
	} {
		if len(value) == 0 {
			t.Errorf("json.Unmarshal(expanded resource).%s is empty, want JSON", name)
		}
	}
}

func TestIssueListPushesQueryAndLimit(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"2-1","idReadable":"DEMO-1","summary":"one"}]`))
	}))
	t.Cleanup(server.Close)

	baseURL, token := server.URL, "test-token"
	connection := &plugin.Connection{}
	connection.SetConfig(Config{BaseURL: &baseURL, Token: &token})
	limit := int64(1)
	var rows []resource
	d := &plugin.QueryData{
		Connection:     connection,
		QueryContext:   &plugin.QueryContext{Limit: &limit},
		EqualsQuals:    plugin.KeyColumnEqualsQualMap{"query": proto.NewQualValue("project: DEMO")},
		StreamListItem: func(_ context.Context, items ...any) { rows = append(rows, items[0].(resource)) },
	}
	definition := resourceDefinitions()[1]
	if _, err := resourceList(definition)(context.Background(), d, nil); err != nil {
		t.Fatalf("resourceList(issue) error = %v, want nil", err)
	}
	if len(rows) != 1 || rows[0].ID != "2-1" {
		t.Errorf("resourceList(issue) rows = %#v, want one row 2-1", rows)
	}
	if got := gotQuery.Get("query"); got != "project: DEMO" {
		t.Errorf("resourceList(issue) query = %q, want %q", got, "project: DEMO")
	}
	if got := gotQuery.Get("$top"); got != "1" {
		t.Errorf("resourceList(issue) $top = %q, want 1", got)
	}
}

func TestProjectListRequestsOnlySupportedFields(t *testing.T) {
	t.Parallel()

	const wantFields = "id,name,shortName,description,leader(id,login,fullName)"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["fields"]; len(got) != 1 || got[0] != wantFields {
			t.Errorf("resourceList(project) fields = %v, want [%s]", got, wantFields)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	d := queryData(t, server.URL)
	if _, err := resourceList(resourceDefinitions()[0])(context.Background(), d, nil); err != nil {
		t.Fatalf("resourceList(project) error = %v, want nil", err)
	}
}

func TestIssueListByProjectUsesProjectCollection(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	d := queryData(t, server.URL)
	d.EqualsQuals["project_id"] = proto.NewQualValue("project/with space")
	if _, err := resourceList(resourceDefinitions()[1])(context.Background(), d, nil); err != nil {
		t.Fatalf("resourceList(issue with project_id) error = %v, want nil", err)
	}
	if gotPath != "/api/admin/projects/project%2Fwith%20space/issues" {
		t.Errorf("resourceList(issue with project_id) path = %q, want %q", gotPath, "/api/admin/projects/project%2Fwith%20space/issues")
	}
	if gotQuery != "" {
		t.Errorf("resourceList(issue with project_id) query = %q, want empty", gotQuery)
	}
}

func TestWorkItemListPushesDocumentedFilters(t *testing.T) {
	var requests []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	stamp := timestamppb.New(time.Date(2026, time.March, 4, 10, 0, 0, 0, time.UTC))
	d := queryData(t, server.URL)
	d.EqualsQuals = plugin.KeyColumnEqualsQualMap{
		"query": proto.NewQualValue("project: DEMO"), "start_date": proto.NewQualValue(stamp),
		"end_date": proto.NewQualValue(stamp), "start": proto.NewQualValue(stamp),
		"end": proto.NewQualValue(stamp), "created_start": proto.NewQualValue(stamp),
		"created_end": proto.NewQualValue(stamp), "updated_start": proto.NewQualValue(stamp),
		"updated_end": proto.NewQualValue(stamp), "author_filter": qualValueList("alice", "1-2"),
	}
	definition := resourceDefinitions()[9]
	if _, err := resourceList(definition)(context.Background(), d, nil); err != nil {
		t.Fatalf("resourceList(work items) error = %v, want nil", err)
	}

	d.EqualsQuals = plugin.KeyColumnEqualsQualMap{"creator_filter": proto.NewQualValue("bob")}
	if _, err := resourceList(definition)(context.Background(), d, nil); err != nil {
		t.Fatalf("resourceList(work items scalar creator) error = %v, want nil", err)
	}
	if len(requests) != 2 || requests[0].Get("startDate") != "2026-03-04" || requests[0]["author"][1] != "1-2" || requests[1].Get("creator") != "bob" {
		t.Errorf("resourceList(work items) queries = %v, want date, repeated author, and scalar creator filters", requests)
	}
}

func TestWorkItemListByIssueUsesNestedCollection(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	d := queryData(t, server.URL)
	d.EqualsQuals["issue_id"] = proto.NewQualValue("DEMO/7")
	if _, err := resourceList(resourceDefinitions()[9])(context.Background(), d, nil); err != nil {
		t.Fatalf("resourceList(work items by issue) error = %v, want nil", err)
	}
	if gotPath != "/api/issues/DEMO%2F7/timeTracking/workItems" {
		t.Errorf("resourceList(work items by issue) path = %q, want nested escaped collection", gotPath)
	}
}

func qualValueList(values ...string) *proto.QualValue {
	items := make([]*proto.QualValue, len(values))
	for i, value := range values {
		items[i] = proto.NewQualValue(value)
	}
	return &proto.QualValue{Value: &proto.QualValue_ListValue{ListValue: &proto.QualValueList{Values: items}}}
}

func TestResourceHydrateBranches(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "missing"):
			http.NotFound(w, r)
		case strings.Contains(r.URL.Path, "failure"):
			http.Error(w, `{"error":"failed"}`, http.StatusBadRequest)
		default:
			_, _ = w.Write([]byte(`{"id":"2-1","project":{"id":"0-1"},"issue":{"id":"2-1"}}`))
		}
	}))
	t.Cleanup(server.Close)

	d := queryData(t, server.URL)
	definition := resourceDefinitions()[0]
	for _, id := range []string{"2-1", "missing", "failure", ""} {
		d.EqualsQuals = plugin.KeyColumnEqualsQualMap{"id": proto.NewQualValue(id)}
		got, err := resourceGet(definition)(context.Background(), d, nil)
		if id == "failure" {
			if err == nil {
				t.Errorf("resourceGet(%q) error = nil, want error", id)
			}
			continue
		}
		if err != nil {
			t.Errorf("resourceGet(%q) error = %v, want nil", id, err)
		}
		if id == "2-1" && got.(resource).ID != id {
			t.Errorf("resourceGet(%q).ID = %q, want %q", id, got.(resource).ID, id)
		}
		if (id == "missing" || id == "") && got != nil {
			t.Errorf("resourceGet(%q) = %#v, want nil", id, got)
		}
	}

	comment := resourceDefinitions()[8]
	d.EqualsQuals = plugin.KeyColumnEqualsQualMap{"id": proto.NewQualValue("4-1")}
	if got, err := resourceGet(comment)(context.Background(), d, nil); err != nil || got != nil {
		t.Errorf("resourceGet(comment without issue_id) = %#v, %v, want nil, nil", got, err)
	}
	d.EqualsQuals["issue_id"] = proto.NewQualValue("2-1")
	if got, err := resourceGet(comment)(context.Background(), d, nil); err != nil || got.(resource).ID != "2-1" {
		t.Errorf("resourceGet(comment) = %#v, %v, want resource 2-1", got, err)
	}
}

func TestResourceListAndClientErrors(t *testing.T) {
	t.Parallel()

	d := &plugin.QueryData{Connection: &plugin.Connection{}, QueryContext: &plugin.QueryContext{}}
	d.EqualsQuals = plugin.KeyColumnEqualsQualMap{"id": proto.NewQualValue("2-1")}
	if _, err := resourceGet(resourceDefinitions()[0])(context.Background(), d, nil); err == nil {
		t.Error("resourceGet(invalid config) error = nil, want error")
	}
	if _, err := resourceList(resourceDefinitions()[0])(context.Background(), d, nil); err == nil {
		t.Error("resourceList(invalid config) error = nil, want error")
	}
	d.Connection.SetConfig(Config{})
	if _, err := resourceList(resourceDefinitions()[0])(context.Background(), d, nil); err == nil {
		t.Error("resourceList(incomplete config) error = nil, want error")
	}

	d = queryData(t, "http://127.0.0.1:1")
	if _, err := resourceList(resourceDefinitions()[0])(context.Background(), d, nil); err == nil {
		t.Error("resourceList(unavailable) error = nil, want error")
	}

	d = queryData(t, "http://127.0.0.1:1")
	d.EqualsQuals = plugin.KeyColumnEqualsQualMap{"project_id": proto.NewQualValue("0-1")}
	if _, err := resourceList(resourceDefinitions()[1])(context.Background(), d, nil); err == nil {
		t.Error("resourceList(issue project) error = nil, want unavailable error")
	}

	d = queryData(t, "http://127.0.0.1:1")
	if got, err := resourceList(resourceDefinitions()[8])(context.Background(), d, nil); err != nil || got != nil {
		t.Errorf("resourceList(comment without parent) = %#v, %v, want nil, nil", got, err)
	}
}

func TestResourceUtilities(t *testing.T) {
	t.Parallel()

	if got := nestedID(nil); got != "" {
		t.Errorf("nestedID(nil) = %q, want empty", got)
	}
	if got := nestedID(json.RawMessage(`{`)); got != "" {
		t.Errorf("nestedID(malformed) = %q, want empty", got)
	}
	if got := nestedID(json.RawMessage(`{"id":"2-1"}`)); got != "2-1" {
		t.Errorf("nestedID(valid) = %q, want 2-1", got)
	}
	if got, err := milliseconds(context.Background(), &transform.TransformData{Value: "1720000000123"}); err != nil || got.(time.Time).UnixMilli() != 1720000000123 {
		t.Errorf("milliseconds(string) = %v, %v, want Unix milliseconds", got, err)
	}
	var value *int64
	if got, err := milliseconds(context.Background(), &transform.TransformData{Value: value}); err != nil || got != nil {
		t.Errorf("milliseconds(nil pointer) = %v, %v, want nil, nil", got, err)
	}
}

func queryData(t *testing.T, baseURL string) *plugin.QueryData {
	t.Helper()
	token := "test-token"
	connection := &plugin.Connection{}
	connection.SetConfig(Config{BaseURL: &baseURL, Token: &token})
	return &plugin.QueryData{Connection: connection, QueryContext: &plugin.QueryContext{}, EqualsQuals: make(plugin.KeyColumnEqualsQualMap), StreamListItem: func(context.Context, ...any) {}}
}

func TestMilliseconds(t *testing.T) {
	t.Parallel()

	value := int64(1720000000123)
	got, err := milliseconds(context.Background(), &transform.TransformData{Value: &value})
	if err != nil {
		t.Fatalf("milliseconds(%d) error = %v, want nil", value, err)
	}
	want := time.UnixMilli(value).UTC()
	if !got.(time.Time).Equal(want) {
		t.Errorf("milliseconds(%d) = %v, want %v", value, got, want)
	}

	got, err = milliseconds(context.Background(), &transform.TransformData{})
	if err != nil || got != nil {
		t.Errorf("milliseconds(nil) = %v, %v, want nil, nil", got, err)
	}
	if _, err := milliseconds(context.Background(), &transform.TransformData{Value: "invalid"}); err == nil {
		t.Error("milliseconds(invalid) error = nil, want error")
	}
}
