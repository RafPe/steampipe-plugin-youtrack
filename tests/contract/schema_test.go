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
		"youtrack_project": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"name": proto.ColumnType_STRING, "short_name": proto.ColumnType_STRING,
			"description": proto.ColumnType_STRING, "leader": proto.ColumnType_JSON,
		},
		"youtrack_issue": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"id_readable": proto.ColumnType_STRING, "summary": proto.ColumnType_STRING,
			"description": proto.ColumnType_STRING, "project": proto.ColumnType_JSON,
			"project_id": proto.ColumnType_STRING, "reporter": proto.ColumnType_JSON,
			"updater": proto.ColumnType_JSON, "created": proto.ColumnType_TIMESTAMP,
			"updated": proto.ColumnType_TIMESTAMP, "resolved": proto.ColumnType_TIMESTAMP,
			"is_draft": proto.ColumnType_BOOL, "tags": proto.ColumnType_JSON,
			"custom_fields": proto.ColumnType_JSON, "comments_count": proto.ColumnType_INT,
			"votes": proto.ColumnType_INT, "query": proto.ColumnType_STRING,
		},
		"youtrack_user": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"name": proto.ColumnType_STRING, "login": proto.ColumnType_STRING,
			"full_name": proto.ColumnType_STRING, "email": proto.ColumnType_STRING,
			"banned": proto.ColumnType_BOOL, "online": proto.ColumnType_BOOL,
		},
		"youtrack_group": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"name": proto.ColumnType_STRING, "description": proto.ColumnType_STRING,
			"users": proto.ColumnType_JSON, "query": proto.ColumnType_STRING,
		},
		"youtrack_tag": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"name": proto.ColumnType_STRING, "owner": proto.ColumnType_JSON,
			"untag_on_resolve":        proto.ColumnType_BOOL,
			"read_sharing_settings":   proto.ColumnType_JSON,
			"tag_sharing_settings":    proto.ColumnType_JSON,
			"update_sharing_settings": proto.ColumnType_JSON,
			"query":                   proto.ColumnType_STRING,
		},
		"youtrack_saved_query": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"name": proto.ColumnType_STRING, "query_text": proto.ColumnType_STRING,
			"owner": proto.ColumnType_JSON, "read_sharing_settings": proto.ColumnType_JSON,
			"update_sharing_settings": proto.ColumnType_JSON,
		},
		"youtrack_article": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"id_readable": proto.ColumnType_STRING, "summary": proto.ColumnType_STRING,
			"content": proto.ColumnType_STRING, "project": proto.ColumnType_JSON,
			"reporter": proto.ColumnType_JSON, "created": proto.ColumnType_TIMESTAMP,
			"updated": proto.ColumnType_TIMESTAMP, "tags": proto.ColumnType_JSON,
		},
		"youtrack_agile": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"name": proto.ColumnType_STRING, "owner": proto.ColumnType_JSON,
			"projects": proto.ColumnType_JSON, "sprints": proto.ColumnType_JSON,
			"current_sprint": proto.ColumnType_JSON,
		},
		"youtrack_issue_comment": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"issue_id": proto.ColumnType_STRING, "text": proto.ColumnType_STRING,
			"author": proto.ColumnType_JSON, "issue": proto.ColumnType_JSON,
			"created": proto.ColumnType_TIMESTAMP, "updated": proto.ColumnType_TIMESTAMP,
		},
		"youtrack_issue_work_item": {
			"id": proto.ColumnType_STRING, "raw": proto.ColumnType_JSON,
			"issue_id": proto.ColumnType_STRING, "text": proto.ColumnType_STRING,
			"author": proto.ColumnType_JSON, "creator": proto.ColumnType_JSON,
			"issue": proto.ColumnType_JSON, "duration": proto.ColumnType_JSON,
			"date": proto.ColumnType_TIMESTAMP, "created": proto.ColumnType_TIMESTAMP,
			"updated": proto.ColumnType_TIMESTAMP, "query": proto.ColumnType_STRING,
			"start_date": proto.ColumnType_TIMESTAMP, "end_date": proto.ColumnType_TIMESTAMP,
			"start": proto.ColumnType_TIMESTAMP, "end": proto.ColumnType_TIMESTAMP,
			"created_start": proto.ColumnType_TIMESTAMP, "created_end": proto.ColumnType_TIMESTAMP,
			"updated_start": proto.ColumnType_TIMESTAMP, "updated_end": proto.ColumnType_TIMESTAMP,
			"author_filter": proto.ColumnType_STRING, "creator_filter": proto.ColumnType_STRING,
		},
	}

	tables := youtrack.Plugin(context.Background()).TableMap
	for tableName, columns := range want {
		table := tables[tableName]
		if table == nil {
			t.Errorf("Plugin().TableMap[%q] = nil, want table", tableName)
			continue
		}
		if got := len(table.Columns); got != len(columns) {
			t.Errorf("%s column count = %d, want %d", tableName, got, len(columns))
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
