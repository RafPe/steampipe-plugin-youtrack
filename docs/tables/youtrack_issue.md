# youtrack_issue

Returns issues visible to the permanent-token user. Nested and polymorphic values remain JSONB, and `raw` preserves the requested API representation.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | YouTrack database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `id_readable` | text | Empty if omitted by YouTrack. | Human-readable issue ID, such as `DEMO-7`. |
| `summary` | text | Empty if omitted by YouTrack. | Issue summary. |
| `description` | text | Null when unset. | Issue description. |
| `project` | jsonb | Null when not visible. | Project object containing its ID, name, and short name. |
| `project_id` | text | Empty when the project object is unavailable. | Database ID derived from `project`; exact equality selects the project-scoped endpoint. |
| `reporter` | jsonb | Null when unset or not visible. | Reporter object. |
| `updater` | jsonb | Null when unset or not visible. | Most recent updater object. |
| `created` | timestamp with time zone | Null when unavailable. | Issue creation time. |
| `updated` | timestamp with time zone | Null when unavailable. | Last issue update time. |
| `resolved` | timestamp with time zone | Null while unresolved. | Resolution time. |
| `is_draft` | boolean | False if omitted by YouTrack. | Whether the issue is a draft. |
| `tags` | jsonb | Null when unavailable; may be an empty array. | Tags attached to the issue. |
| `custom_fields` | jsonb | Null when unavailable; may be an empty array. | Raw polymorphic custom-field values retained for forward compatibility. |
| `comments_count` | bigint | Zero if omitted by YouTrack. | Number of comments. |
| `votes` | bigint | Zero if omitted by YouTrack. | Number of votes. |
| `query` | text | Null unless supplied as a qualifier. | Control column containing the exact YouTrack issue-search expression sent to the API. |

## Querying

Exact `id` or `id_readable` equality uses the single-issue endpoint. Exact `project_id` uses the project-scoped issue collection. Exact `query` is passed verbatim to YouTrack; it uses YouTrack search syntax, not SQL semantics. Other SQL predicates remain local.

```sql
select id_readable, summary, project ->> 'shortName' as project, updated
from youtrack_issue
where query = 'project: DEMO State: Unresolved'
order by updated desc
limit 20;
```

```sql
select id, id_readable, summary, reporter
from youtrack_issue
where id_readable = 'DEMO-7';
```

## Permissions

Specific issue access requires **Read Issue**, subject to issue visibility and YouTrack's documented reporter exception. Collection results contain only issues visible to the token user.
