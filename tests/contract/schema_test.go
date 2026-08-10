// Package contract verifies the public SQL schema exposed by the plugin.
package contract

import (
	"context"
	"testing"

	"github.com/RafPe/steampipe-youtrack/youtrack"
	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
)

func TestPublicSchema(t *testing.T) {
	t.Parallel()

	want := map[string]map[string]proto.ColumnType{
		"youtrack_project":         {"id": proto.ColumnType_STRING, "short_name": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON},
		"youtrack_issue":           {"id": proto.ColumnType_STRING, "id_readable": proto.ColumnType_STRING, "created": proto.ColumnType_TIMESTAMP, "custom_fields": proto.ColumnType_JSON},
		"youtrack_user":            {"id": proto.ColumnType_STRING, "login": proto.ColumnType_STRING},
		"youtrack_group":           {"id": proto.ColumnType_STRING, "name": proto.ColumnType_STRING},
		"youtrack_tag":             {"id": proto.ColumnType_STRING, "owner": proto.ColumnType_JSON},
		"youtrack_saved_query":     {"id": proto.ColumnType_STRING, "owner": proto.ColumnType_JSON},
		"youtrack_article":         {"id": proto.ColumnType_STRING, "content": proto.ColumnType_STRING},
		"youtrack_agile":           {"id": proto.ColumnType_STRING, "owner": proto.ColumnType_JSON},
		"youtrack_issue_comment":   {"id": proto.ColumnType_STRING, "issue_id": proto.ColumnType_STRING},
		"youtrack_issue_work_item": {"id": proto.ColumnType_STRING, "issue_id": proto.ColumnType_STRING},
	}

	tables := youtrack.Plugin(context.Background()).TableMap
	for tableName, columns := range want {
		table := tables[tableName]
		if table == nil {
			t.Errorf("Plugin().TableMap[%q] = nil, want table", tableName)
			continue
		}
		for columnName, columnType := range columns {
			found := false
			for _, column := range table.Columns {
				if column.Name == columnName {
					found = true
					if column.Type != columnType {
						t.Errorf("%s.%s type = %s, want %s", tableName, columnName, column.Type, columnType)
					}
				}
			}
			if !found {
				t.Errorf("%s.%s missing from public schema", tableName, columnName)
			}
		}
	}
}
