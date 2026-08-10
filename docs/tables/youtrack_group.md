# youtrack_group

Returns user groups visible through YouTrack's current `/api/groups` resource.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | Group database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `name` | text | Empty if omitted by YouTrack. | Group name. |
| `description` | text | Null when unset or not visible. | Group description. |
| `users` | jsonb | Null when not visible; may be an empty array. | Visible users in the group. |
| `query` | text | Null unless supplied as a qualifier. | Control column containing the exact group query sent to YouTrack. |

## Querying

Exact `id` equality uses the single-group endpoint; only database IDs are accepted. Exact `query` is passed verbatim to the collection endpoint. Group-name equality is not an identifier lookup and remains local.

```sql
select id, name, description
from youtrack_group
where query = 'developers'
order by name;
```

```sql
select g.name, member.value ->> 'login' as member_login
from youtrack_group g
cross join lateral jsonb_array_elements(g.users) as member(value)
where g.id = '1-2';
```

## Permissions

Listing requires **Read Groups**, **Update Project**, or **Low-Level Admin Read**. The group's **Visible to** setting can further restrict results and membership data.
