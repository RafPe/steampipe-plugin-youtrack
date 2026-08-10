# youtrack_issue_work_item

Returns issue work items visible to the permanent-token user. Missing optional YouTrack values are SQL null; nested or polymorphic values remain JSONB and `raw` preserves the requested representation.

`issue_id` uses the nested issue work-item collection. Without it, the plugin uses the global work-item endpoint, where `query`, `start_date`, `end_date`, `start`, `end`, `created_start`, `created_end`, `updated_start`, `updated_end`, `author_filter`, and `creator_filter` are pushed exactly as documented. Author and creator filters accept one value with `=` or repeated values with `in`; the result-bearing `author` and `creator` columns remain JSONB.

## Columns

Every row includes `id`, resource-specific stable scalar columns, nested JSONB where flattening would lose information, and `raw`. Timestamp columns are converted from YouTrack Unix milliseconds to PostgreSQL timestamps.

## Examples

```sql
select id, text, author, duration
from youtrack_issue_work_item
where issue_id = 'DEMO-7';
```

```sql
select id, issue, author, date, duration
from youtrack_issue_work_item
where author_filter in ('rafal.pieniazek', '1-2')
  and start_date = timestamp '2026-08-01'
  and end_date = timestamp '2026-08-10'
limit 20;
```
