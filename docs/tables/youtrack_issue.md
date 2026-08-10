# youtrack_issue

Returns issues visible to the permanent-token user. Missing optional YouTrack values are SQL null; nested or polymorphic values remain JSONB and `raw` preserves the requested representation.

The `id` and `id_readable` qualifiers use the single-resource endpoint. Collection reads use `$top`/`$skip`, honor SQL limits and cancellation, and require the corresponding YouTrack read permission. The optional `query` qualifier is pushed to YouTrack verbatim. `project_id` uses YouTrack's project-scoped issue collection. Other SQL predicates remain local because their SQL semantics are not documented as equivalent to YouTrack search.

## Columns

Every row includes `id`, resource-specific stable scalar columns, nested JSONB where flattening would lose information, and `raw`. Timestamp columns are converted from YouTrack Unix milliseconds to PostgreSQL timestamps.

## Examples

```sql
select * from youtrack_issue where id = 'example-id';
```

```sql
select id_readable, summary, updated
from youtrack_issue
where query = 'project: DEMO State: Unresolved'
order by updated desc
limit 20;
```
