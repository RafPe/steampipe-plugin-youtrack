# youtrack_tag

Returns tags visible to the permanent-token user, including their separate visibility, use, and update sharing settings.

## Columns

| Column | Type | Nullable behavior | Description |
| --- | --- | --- | --- |
| `id` | text | Normally non-null. | Tag database ID. |
| `raw` | jsonb | Normally non-null. | Complete requested YouTrack representation. |
| `name` | text | Empty if omitted by YouTrack. | Tag name. |
| `owner` | jsonb | Null when unset or not visible. | Tag owner object. |
| `untag_on_resolve` | boolean | Null when not visible. | Whether YouTrack removes the tag when an issue is resolved. |
| `read_sharing_settings` | jsonb | Null when unavailable. | Settings controlling tag visibility. |
| `tag_sharing_settings` | jsonb | Null when unavailable. | Settings controlling who may use the tag. |
| `update_sharing_settings` | jsonb | Null when unavailable. | Settings controlling who may update the tag. |
| `query` | text | Null unless supplied as a qualifier. | Control column containing the exact tag-name search sent to YouTrack. |

## Querying

Exact `id` equality uses the single-tag endpoint; only database IDs are accepted. Exact `query` is passed verbatim to YouTrack's tag-name search. `name =` remains a local predicate because the API search is not documented as exact equality.

```sql
select id, name, owner, untag_on_resolve
from youtrack_tag
where query = 'release'
order by name;
```

```sql
select name, read_sharing_settings, tag_sharing_settings
from youtrack_tag
where id = '6-42';
```

## Permissions

Lists contain tags visible to the current user. A specific tag is visible to its owner or users covered by its sharing configuration; visibility, use, and update access are independent.
