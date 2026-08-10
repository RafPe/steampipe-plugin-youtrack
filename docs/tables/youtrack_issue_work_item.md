# youtrack_issue_work_item

Returns visible work items through either the global collection or an issue-scoped collection. Result objects remain JSONB, while YouTrack millisecond timestamps are converted to PostgreSQL timestamps.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | Work-item database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `issue_id` | text | Empty when the nested issue is unavailable. | Parent issue database ID; as a qualifier it selects the nested collection. |
| `text` | text | Null when unset. | Work-item text. |
| `author` | jsonb | Null when not visible. | Work-item author object. |
| `creator` | jsonb | Null when not visible. | User who created the work item. |
| `issue` | jsonb | Null when not visible. | Parent issue object. |
| `duration` | jsonb | Null when unavailable. | Recorded duration, including minutes and presentation. |
| `date` | timestamp with time zone | Null when unavailable. | Work date; YouTrack represents it at midnight. |
| `created` | timestamp with time zone | Null when unavailable. | Work-item creation time. |
| `updated` | timestamp with time zone | Null when unavailable. | Last work-item update time. |
| `query` | text | Null unless supplied as a qualifier. | Control column containing an exact YouTrack issue-search query for the global endpoint. |
| `start_date` | timestamp with time zone | Null unless supplied as a qualifier. | Inclusive work-date lower bound, sent as `YYYY-MM-DD`. |
| `end_date` | timestamp with time zone | Null unless supplied as a qualifier. | Inclusive work-date upper bound, sent as `YYYY-MM-DD`. |
| `start` | timestamp with time zone | Null unless supplied as a qualifier. | Work timestamp lower bound, sent as Unix milliseconds. |
| `end` | timestamp with time zone | Null unless supplied as a qualifier. | Work timestamp upper bound, sent as Unix milliseconds. |
| `created_start` | timestamp with time zone | Null unless supplied as a qualifier. | Creation timestamp lower bound, sent as Unix milliseconds. |
| `created_end` | timestamp with time zone | Null unless supplied as a qualifier. | Creation timestamp upper bound, sent as Unix milliseconds. |
| `updated_start` | timestamp with time zone | Null unless supplied as a qualifier. | Update timestamp lower bound, sent as Unix milliseconds. |
| `updated_end` | timestamp with time zone | Null unless supplied as a qualifier. | Update timestamp upper bound, sent as Unix milliseconds. |
| `author_filter` | text | Null unless supplied as a qualifier. | Control column mapped to one or more repeated `author` parameters. |
| `creator_filter` | text | Null unless supplied as a qualifier. | Control column mapped to one or more repeated `creator` parameters. |

## Querying

Exact `issue_id` uses the nested issue work-item collection. Without it, `query`, date and timestamp bounds, `author_filter`, and `creator_filter` use the global endpoint. Author and creator accept a database ID, login, `ringId`, or `me`; use `IN` for repeated values. Exact `id` uses the global single-resource endpoint unless an `issue_id` is also retained. Do not substitute result-bearing `author` or `creator` JSONB columns for their filter columns.

```sql
select id, issue, author, date, duration
from youtrack_issue_work_item
where author_filter = 'me'
  and start_date = timestamp '2026-08-01'
  and end_date = timestamp '2026-08-31'
order by date desc;
```

```sql
select issue ->> 'idReadable' as issue_id,
       sum((duration ->> 'minutes')::integer) as minutes
from youtrack_issue_work_item
where author_filter in ('rafal.pieniazek', '1-2')
group by issue ->> 'idReadable'
order by minutes desc;
```

## Permissions

A specific work item requires access to its parent issue and **Read Work Item**. Global and nested collections contain only work items visible to the token user.
