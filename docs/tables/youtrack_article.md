# youtrack_article

Returns knowledge-base articles visible to the permanent-token user. Missing optional YouTrack values are SQL null; nested or polymorphic values remain JSONB and `raw` preserves the requested representation.

The `id` qualifier uses the single-resource endpoint. Collection reads use `$top`/`$skip`, honor SQL limits and cancellation, and require the corresponding YouTrack read permission.

## Columns

Every row includes `id`, resource-specific stable scalar columns, nested JSONB where flattening would lose information, and `raw`. Timestamp columns are converted from YouTrack Unix milliseconds to PostgreSQL timestamps.

## Examples

```sql
select * from youtrack_article where id = 'example-id';
```

```sql
select id, name from youtrack_article limit 20;
```

