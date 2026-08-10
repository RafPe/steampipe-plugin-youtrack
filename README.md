<p align="center">
  <img src="./assets/social-card.png" alt="steampipe-youtrack — Query JetBrains YouTrack with SQL" width="820">
</p>

# Steampipe Plugin for YouTrack

Query JetBrains YouTrack through PostgreSQL-compatible SQL. The plugin is read-only and supports Steampipe and Turbot Pipes.

## Prerequisites and installation

Install Go 1.26 and Steampipe, then build the plugin with `make build`. For local development, install the binary at `~/.steampipe/plugins/local/youtrack/youtrack.plugin`; Steampipe requires the plugin-specific directory beneath `plugins/local`.

```sh
mkdir -p ~/.steampipe/plugins/local/youtrack
go build -o ~/.steampipe/plugins/local/youtrack/youtrack.plugin .
chmod 700 ~/.steampipe/plugins/local/youtrack/youtrack.plugin
```

Create a permanent token in your YouTrack profile. Treat it as a password: store it in an environment variable, never commit it, and rotate it if exposed.

```hcl
connection "youtrack" {
  plugin   = "youtrack"
  base_url = "https://example.youtrack.cloud"
  token    = env("YOUTRACK_TOKEN")
}
```

Root and sub-path installations are supported; `/api` is appended exactly once. HTTPS is required except for explicit loopback E2E URLs.

## Tables

`youtrack_project`, `youtrack_issue`, `youtrack_user`, `youtrack_group`, `youtrack_tag`, `youtrack_saved_query`, `youtrack_article`, `youtrack_agile`, `youtrack_issue_comment`, and `youtrack_issue_work_item`.

```sql
select id_readable, summary, created
from youtrack_issue
where query = 'project: DEMO #Unresolved';

select p.short_name, count(*)
from youtrack_project p
join youtrack_issue i on i.project_id = p.id
group by p.short_name;
```

Explore the [query cookbook](docs/queries.md) for reporting, JSON extraction,
aggregations, identifier lookups, and multi-table joins.

## Development

Use `make test`, `make test-race`, `make test-contract`, `make coverage`, `make lint`, `make build`, or the complete local CI equivalent `make check`. The coverage gate requires 100% statement coverage in first-party testable packages. See [E2E testing](docs/e2e.md) for the pinned real YouTrack and Steampipe flow.

## Limitations

Results reflect the token user's permissions. YouTrack Server 2026.1 or later is required for the current `/api/users` and `/api/groups` resources; the plugin never falls back to deprecated Hub endpoints. The pinned E2E image requires a one-time supported first-run wizard and manual permanent-token creation.

## Contributing

Add one red-green-refactor slice at a time, document schema changes, keep credentials out of fixtures and logs, and run `make check` before opening a pull request. Contributions are licensed under Apache-2.0.
