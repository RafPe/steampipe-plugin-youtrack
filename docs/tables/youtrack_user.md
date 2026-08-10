# youtrack_user

Returns users visible through YouTrack's current `/api/users` resource. The plugin does not use deprecated Hub endpoints.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | YouTrack database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `name` | text | Empty if omitted by YouTrack. | User name. |
| `login` | text | Empty if omitted by YouTrack. | User login. |
| `full_name` | text | Empty if omitted by YouTrack. | User full name. |
| `email` | text | Null when hidden or unset. | Email address visible to the token user. |
| `banned` | boolean | Null when not visible. | Whether the account is banned. |
| `online` | boolean | Null when not visible. | Whether the account is online. |

## Querying

Exact `id` or `login` equality uses the single-user endpoint; a login is accepted where YouTrack documents `userID`. The users collection has no documented email or name filter, so those predicates remain local and may scan all visible users. The value `me` is treated as a literal login by this table, not as automatic current-user resolution.

```sql
select id, login, full_name, email
from youtrack_user
where login = 'rafal.pieniazek';
```

```sql
select login, full_name, banned, online
from youtrack_user
where email is not null
order by login
limit 20;
```

## Permissions

Basic user information needs no special permission. Reading all profile data requires **Read User**; hidden fields are returned as null.
