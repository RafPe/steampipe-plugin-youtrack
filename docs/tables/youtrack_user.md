# youtrack_user

Returns users visible to the permanent-token user. Missing optional YouTrack values are SQL null; nested or polymorphic values remain JSONB and `raw` preserves the requested representation.

The `id` and `login` qualifiers use the single-resource endpoint. Collection reads use `$top`/`$skip`, honor SQL limits and cancellation, and require the corresponding YouTrack read permission. The current users API does not document an email collection filter, so `email` remains a local predicate and may require scanning every visible user.

## Columns

Every row includes `id`, resource-specific stable scalar columns, nested JSONB where flattening would lose information, and `raw`. Timestamp columns are converted from YouTrack Unix milliseconds to PostgreSQL timestamps.

## Examples

```sql
select * from youtrack_user where id = 'example-id';
```

```sql
select id, login, full_name, email
from youtrack_user
where login = 'rafal.pieniazek';
```
