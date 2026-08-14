package contract

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/RafPe/steampipe-plugin-youtrack/youtrack"
)

// TestTableDocumentationFollowsHubFormat verifies that every table ships a
// doc in the standard Steampipe Hub table-doc format: frontmatter naming the
// table, a matching title heading, a usage guide, and paired
// sql+postgres/sql+sqlite examples.
func TestTableDocumentationFollowsHubFormat(t *testing.T) {
	t.Parallel()

	tables := youtrack.Plugin(context.Background()).TableMap
	documents := os.DirFS("../../docs/tables")
	for tableName := range tables {
		t.Run(tableName, func(t *testing.T) {
			t.Parallel()

			path := tableName + ".md"
			data, err := fs.ReadFile(documents, path)
			if err != nil {
				t.Fatalf("fs.ReadFile(docs, %q) error = %v, want nil", path, err)
			}
			document := string(data)

			if !strings.HasPrefix(document, "---\n") {
				t.Errorf("documentation for %s starts with YAML frontmatter = false, want true", tableName)
			}
			for _, want := range []string{
				fmt.Sprintf("title: \"Steampipe Table: %s - ", tableName),
				"description: \"",
				"folder: \"",
				fmt.Sprintf("# Table: %s - ", tableName),
				"## Table Usage Guide",
				"## Examples",
			} {
				if !strings.Contains(document, want) {
					t.Errorf("documentation for %s contains %q = false, want true", tableName, want)
				}
			}

			postgres := strings.Count(document, "```sql+postgres")
			sqlite := strings.Count(document, "```sql+sqlite")
			if postgres < 2 {
				t.Errorf("documentation for %s sql+postgres example count = %d, want at least 2", tableName, postgres)
			}
			if postgres != sqlite {
				t.Errorf("documentation for %s sql+sqlite example count = %d, want %d (one per sql+postgres example)", tableName, sqlite, postgres)
			}
		})
	}
}
