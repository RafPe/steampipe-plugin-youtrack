# youtrack_saved_query

Returns saved issue searches visible to the permanent-token user. Sharing settings remain JSONB so no access-control information is lost through flattening.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | Saved-query database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `name` | text | Empty if omitted by YouTrack. | Saved-query display name. |
| `query_text` | text | Empty if omitted by YouTrack. | Stored YouTrack issue-search expression. |
| `owner` | jsonb | Null when unset or not visible. | Saved-query owner object. |
| `read_sharing_settings` | jsonb | Null when unavailable. | Settings controlling visibility. |
| `update_sharing_settings` | jsonb | Null when unavailable. | Settings controlling who may update the saved query. |

## Querying

Exact `id` equality uses the single-resource endpoint and accepts the saved query's database ID. The collection has no documented search parameter, so predicates on `name`, `query_text`, owner, or sharing settings remain local.

```sql
select id, name, query_text, owner
from youtrack_saved_query
order by name;
```

```sql
select name, query_text, read_sharing_settings
from youtrack_saved_query
where id = '10-42';
```

## Permissions

The collection contains saved searches visible to the current user. A specific saved search is readable by its author or by members of groups with which it is shared.
