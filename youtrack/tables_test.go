package youtrack

import (
	"context"
	"testing"

	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin/transform"
)

func TestTableSchemas(t *testing.T) {
	t.Parallel()

	want := []string{
		"youtrack_project", "youtrack_issue", "youtrack_user", "youtrack_group",
		"youtrack_tag", "youtrack_saved_query", "youtrack_article", "youtrack_agile",
		"youtrack_issue_comment", "youtrack_issue_work_item",
	}
	tables := tableMap(context.Background())
	for _, name := range want {
		table := tables[name]
		if table == nil {
			t.Errorf("tableMap()[%q] = nil, want table", name)
			continue
		}
		if table.List == nil || table.Get == nil {
			t.Errorf("tableMap()[%q] list/get = %v/%v, want both", name, table.List, table.Get)
		}
		assertColumn(t, table, "id", proto.ColumnType_STRING)
		assertColumn(t, table, "raw", proto.ColumnType_JSON)
	}

	issue := tables["youtrack_issue"]
	for name, columnType := range map[string]proto.ColumnType{
		"id_readable": proto.ColumnType_STRING, "summary": proto.ColumnType_STRING,
		"description": proto.ColumnType_STRING, "created": proto.ColumnType_TIMESTAMP,
		"updated": proto.ColumnType_TIMESTAMP, "resolved": proto.ColumnType_TIMESTAMP,
		"custom_fields": proto.ColumnType_JSON, "tags": proto.ColumnType_JSON,
	} {
		assertColumn(t, issue, name, columnType)
	}

	workItem := tables["youtrack_issue_work_item"]
	for name, columnType := range map[string]proto.ColumnType{
		"query": proto.ColumnType_STRING, "creator": proto.ColumnType_JSON,
		"date":    proto.ColumnType_TIMESTAMP,
		"created": proto.ColumnType_TIMESTAMP, "updated": proto.ColumnType_TIMESTAMP,
		"author_filter": proto.ColumnType_STRING, "creator_filter": proto.ColumnType_STRING,
	} {
		assertColumn(t, workItem, name, columnType)
	}
	for _, name := range []string{"start_date", "end_date", "start", "end", "created_start", "created_end", "updated_start", "updated_end"} {
		for _, column := range workItem.Columns {
			if column.Name == name {
				t.Errorf("table %q still has pseudo-qualifier column %q, want range operators on real timestamp columns", workItem.Name, name)
			}
		}
	}
	rangeColumns := map[string][]string{}
	for _, keyColumn := range workItem.List.KeyColumns {
		rangeColumns[keyColumn.Name] = keyColumn.Operators
	}
	for _, name := range []string{"date", "created", "updated"} {
		want := []string{">", ">=", "=", "<", "<="}
		if got := rangeColumns[name]; len(got) != len(want) {
			t.Errorf("table %q list key column %q operators = %v, want %v", workItem.Name, name, got, want)
		}
	}
}

func TestReadableIDTransforms(t *testing.T) {
	t.Parallel()

	tables := tableMap(context.Background())
	for _, tableName := range []string{"youtrack_issue", "youtrack_article"} {
		column := findColumn(t, tables[tableName], "id_readable")
		if column.Transform == nil {
			t.Errorf("tableMap()[%q] id_readable transform = nil, want explicit IDReadable transform", tableName)
			continue
		}
		got, err := column.Transform.Execute(context.Background(), &transform.TransformData{
			HydrateItem: resource{IDReadable: "DEMO-7"},
			ColumnName:  "id_readable",
		})
		if err != nil {
			t.Errorf("tableMap()[%q] id_readable transform error = %v, want nil", tableName, err)
			continue
		}
		if got != "DEMO-7" {
			t.Errorf("tableMap()[%q] id_readable transform = %v, want DEMO-7", tableName, got)
		}
	}

	for tableName, table := range tables {
		assertTransformValue(t, tableName, findColumn(t, table, "id"), resource{ID: "2-7"}, "2-7")
	}
	assertTransformValue(t, "youtrack_issue", findColumn(t, tables["youtrack_issue"], "project_id"), resource{ProjectID: "0-3"}, "0-3")
	assertTransformValue(t, "youtrack_saved_query", findColumn(t, tables["youtrack_saved_query"], "query_text"), resource{Query: "project: DEMO"}, "project: DEMO")
}

func assertTransformValue(t *testing.T, tableName string, column *plugin.Column, item resource, want string) {
	t.Helper()
	if column.Transform == nil {
		t.Errorf("tableMap()[%q] %s transform = nil, want explicit initialism transform", tableName, column.Name)
		return
	}
	got, err := column.Transform.Execute(context.Background(), &transform.TransformData{HydrateItem: item, ColumnName: column.Name})
	if err != nil {
		t.Errorf("tableMap()[%q] %s transform error = %v, want nil", tableName, column.Name, err)
		return
	}
	if got != want {
		t.Errorf("tableMap()[%q] %s transform = %v, want %s", tableName, column.Name, got, want)
	}
}

func assertColumn(t *testing.T, table *plugin.Table, name string, want proto.ColumnType) {
	t.Helper()
	for _, column := range table.Columns {
		if column.Name == name {
			if column.Type != want {
				t.Errorf("table %q column %q type = %q, want %q", table.Name, name, column.Type, want)
			}
			return
		}
	}
	t.Errorf("table %q column %q missing", table.Name, name)
}

func findColumn(t *testing.T, table *plugin.Table, name string) *plugin.Column {
	t.Helper()
	for _, column := range table.Columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("table %q column %q missing", table.Name, name)
	return nil
}
