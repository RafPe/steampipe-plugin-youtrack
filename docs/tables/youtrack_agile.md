# youtrack_agile

Returns agile boards available to the permanent-token user. Associated projects and sprints remain JSONB to preserve their nested representation.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | Agile-board database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `name` | text | Empty if omitted by YouTrack. | Board name. |
| `owner` | jsonb | Null when unset or not visible. | Board owner object. |
| `projects` | jsonb | Null when unavailable; may be an empty array. | Projects associated with the board. |
| `sprints` | jsonb | Null when unavailable; may be an empty array. | Sprints visible on the board. |
| `current_sprint` | jsonb | Null when there is no current sprint or it is unavailable. | Current sprint object. |

## Querying

Exact `id` equality uses the single-board endpoint and accepts only the database ID. The collection has no documented search parameter, so name, owner, project, and sprint predicates remain local.

```sql
select id, name, owner, current_sprint
from youtrack_agile
order by name;
```

```sql
select a.name, project.value ->> 'shortName' as project
from youtrack_agile a
cross join lateral jsonb_array_elements(a.projects) as project(value)
where a.id = '120-4';
```

## Permissions

The collection returns available boards. A specific board requires permission to read that board, so results are access-dependent.
