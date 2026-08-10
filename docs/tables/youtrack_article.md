# youtrack_article

Returns knowledge-base articles visible to the permanent-token user. Nested projects, reporters, and tags remain JSONB, and timestamps are converted from Unix milliseconds.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | Article database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `id_readable` | text | Empty if omitted by YouTrack. | Human-readable article ID, such as `NP-A-1`. |
| `summary` | text | Null when null or omitted by YouTrack. | Article summary. |
| `content` | text | Null when unset. | Article content. |
| `project` | jsonb | Null when unavailable. | Parent project object. |
| `reporter` | jsonb | Null when unset or not visible. | Article reporter object. |
| `created` | timestamp with time zone | Null when unavailable. | Article creation time. |
| `updated` | timestamp with time zone | Null when unavailable. | Last article update time. |
| `tags` | jsonb | Null when unavailable; may be an empty array. | Tags attached to the article. |

## Querying

Exact `id` or `id_readable` equality uses the single-article endpoint. Database IDs and readable article IDs are accepted. The collection documents no global filter, so predicates on summary, content, project, reporter, tags, and timestamps remain local.

```sql
select id_readable, summary, project ->> 'shortName' as project, updated
from youtrack_article
order by updated desc
limit 20;
```

```sql
select id, id_readable, summary, content
from youtrack_article
where id_readable = 'NP-A-1';
```

## Permissions

Article visibility restrictions apply. Specific access is available to permitted users and groups, users with **Override Visibility Restrictions**, and the reporter exception documented by YouTrack.
