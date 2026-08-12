package contract

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/RafPe/steampipe-plugin-youtrack/youtrack"
	"github.com/turbot/steampipe-plugin-sdk/v6/grpc/proto"
)

func TestTableDocumentationMatchesPublicSchema(t *testing.T) {
	t.Parallel()

	tables := youtrack.Plugin(context.Background()).TableMap
	documents := os.DirFS("../../docs/tables")
	for tableName, table := range tables {
		tableName := tableName
		table := table
		t.Run(tableName, func(t *testing.T) {
			t.Parallel()

			path := tableName + ".md"
			data, err := fs.ReadFile(documents, path)
			if err != nil {
				t.Fatalf("fs.ReadFile(docs, %q) error = %v, want nil", path, err)
			}
			document := string(data)
			for _, column := range table.Columns {
				want := fmt.Sprintf("| `%s` | %s |", column.Name, documentedType(column.Type))
				if !strings.Contains(document, want) {
					t.Errorf("documentation for %s column %q does not contain %q", tableName, column.Name, want)
				}
			}
			if got := strings.Count(document, "```sql"); got < 2 {
				t.Errorf("documentation for %s SQL example count = %d, want at least 2", tableName, got)
			}
			for _, heading := range []string{"## Columns", "## Querying", "## Permissions"} {
				if !strings.Contains(document, heading) {
					t.Errorf("documentation for %s contains heading %q = false, want true", tableName, heading)
				}
			}
		})
	}
}

func documentedType(columnType proto.ColumnType) string {
	switch columnType {
	case proto.ColumnType_STRING:
		return "text"
	case proto.ColumnType_INT:
		return "bigint"
	case proto.ColumnType_BOOL:
		return "boolean"
	case proto.ColumnType_TIMESTAMP:
		return "timestamp with time zone"
	case proto.ColumnType_JSON:
		return "jsonb"
	default:
		return "unknown"
	}
}
