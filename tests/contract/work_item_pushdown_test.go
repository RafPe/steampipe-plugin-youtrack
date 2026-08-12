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
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWorkItemPushdownRoutesAndParameters(t *testing.T) {
	t.Parallel()

	t.Run("issue collection", func(t *testing.T) {
		got := invokeWorkItems(t, plugin.KeyColumnEqualsQualMap{
			"issue_id": proto.NewQualValue("DEMO/7"),
		})
		if got.URL.EscapedPath() != "/api/issues/DEMO%2F7/timeTracking/workItems" {
			t.Errorf("work item issue_id request path = %q, want %q", got.URL.EscapedPath(), "/api/issues/DEMO%2F7/timeTracking/workItems")
		}
	})

	t.Run("global documented filters", func(t *testing.T) {
		start := time.Date(2026, time.March, 4, 12, 13, 14, 123000000, time.FixedZone("test", 2*60*60))
		end := start.Add(24 * time.Hour)
		got := invokeWorkItems(t, plugin.KeyColumnEqualsQualMap{
			"query":          proto.NewQualValue("project: DEMO"),
			"start_date":     proto.NewQualValue(timestamppb.New(start)),
			"end_date":       proto.NewQualValue(timestamppb.New(end)),
			"start":          proto.NewQualValue(timestamppb.New(start)),
			"end":            proto.NewQualValue(timestamppb.New(end)),
			"created_start":  proto.NewQualValue(timestamppb.New(start)),
			"created_end":    proto.NewQualValue(timestamppb.New(end)),
			"updated_start":  proto.NewQualValue(timestamppb.New(start)),
			"updated_end":    proto.NewQualValue(timestamppb.New(end)),
			"author_filter":  qualList("alice", "1-2"),
			"creator_filter": qualList("bob", "me"),
		})
		want := url.Values{
			"query": {"project: DEMO"}, "startDate": {"2026-03-04"}, "endDate": {"2026-03-05"},
			"start": {"1772619194123"}, "end": {"1772705594123"},
			"createdStart": {"1772619194123"}, "createdEnd": {"1772705594123"},
			"updatedStart": {"1772619194123"}, "updatedEnd": {"1772705594123"},
			"author": {"alice", "1-2"}, "creator": {"bob", "me"},
		}
		if got.URL.Path != "/api/workItems" {
			t.Errorf("global work item request path = %q, want %q", got.URL.Path, "/api/workItems")
		}
		for key, values := range want {
			if actual := got.URL.Query()[key]; !reflect.DeepEqual(actual, values) {
				t.Errorf("global work item query parameter %q = %v, want %v", key, actual, values)
			}
		}
	})
}

func qualList(values ...string) *proto.QualValue {
	items := make([]*proto.QualValue, len(values))
	for i, value := range values {
		items[i] = proto.NewQualValue(value)
	}
	return &proto.QualValue{Value: &proto.QualValue_ListValue{ListValue: &proto.QualValueList{Values: items}}}
}

func invokeWorkItems(t *testing.T, quals plugin.KeyColumnEqualsQualMap) *http.Request {
	t.Helper()
	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	baseURL, token := server.URL+"/api", "contract-token"
	connection := &plugin.Connection{}
	connection.SetConfig(youtrack.Config{BaseURL: &baseURL, Token: &token})
	table := youtrack.Plugin(context.Background()).TableMap["youtrack_issue_work_item"]
	_, err := table.List.Hydrate(context.Background(), &plugin.QueryData{
		Connection: connection, QueryContext: &plugin.QueryContext{}, EqualsQuals: quals,
		StreamListItem: func(context.Context, ...any) {},
	}, nil)
	if err != nil {
		t.Fatalf("work item list hydrate error = %v, want nil", err)
	}

	select {
	case request := <-requests:
		return request
	case <-time.After(500 * time.Millisecond):
		t.Fatal("work item list hydrate sent no request")
		return nil
	}
}
