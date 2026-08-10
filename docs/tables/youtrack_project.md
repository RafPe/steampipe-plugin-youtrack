# youtrack_project

Returns projects visible to the permanent-token user. Nested objects remain JSONB, and `raw` preserves the requested API representation.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | YouTrack database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `name` | text | Empty if omitted by YouTrack. | Project display name. |
| `short_name` | text | Empty if omitted by YouTrack. | Project short name, such as `DEMO`. |
| `description` | text | Null when unset or not visible. | Project description. |
| `leader` | jsonb | Null when unset or not visible. | Project leader object. |

## Querying

Exact `id` or `short_name` equality uses `GET /api/admin/projects/{projectID}`. Both database IDs and project short names are accepted. Other predicates remain local. Collection reads use `$top` and `$skip` and honor the SQL limit.

```sql
select id, name, short_name, leader
from youtrack_project
order by short_name;
```

```sql
select id, name, description
from youtrack_project
where short_name = 'DEMO';
```

## Permissions

A specific project requires **Read Project Basic** or **Update Project**. Collection results include only projects available to the token user.
