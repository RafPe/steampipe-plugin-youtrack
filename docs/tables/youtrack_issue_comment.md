# youtrack_issue_comment

Returns comments for one accessible issue. YouTrack provides no global comments collection, so `issue_id` is required.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | Comment database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `issue_id` | text | Required for every query; returned from the qualifier. | Parent issue database ID or readable issue ID. |
| `text` | text | Null when unset or not visible. | Comment text. |
| `author` | jsonb | Null when unset or not visible. | Comment author object. |
| `issue` | jsonb | Null when unavailable. | Parent issue object. |
| `created` | timestamp with time zone | Null when unavailable. | Comment creation time. |
| `updated` | timestamp with time zone | Null when never updated or unavailable. | Last comment update time. |

## Querying

Exact `issue_id` selects `/api/issues/{issueID}/comments`; it accepts the parent issue identifier understood by YouTrack, including a database ID or readable issue ID. A single comment requires both `issue_id` and its database `id`. Text, author, and timestamp predicates remain local.

```sql
select id, text, author, created
from youtrack_issue_comment
where issue_id = 'DEMO-7'
order by created;
```

```sql
select id, issue_id, text, updated
from youtrack_issue_comment
where issue_id = 'DEMO-7'
  and id = '4-42';
```

## Permissions

Reading comments requires access to the parent issue. An author can always see their own comment, while comment visibility settings can restrict other readers.
