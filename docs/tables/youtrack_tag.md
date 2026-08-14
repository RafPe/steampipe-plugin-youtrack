---
title: "Steampipe Table: youtrack_tag - Query YouTrack Tags using SQL"
description: "Allows users to query YouTrack Tags, providing tag names, owners, and the independent sharing settings that control who can see, apply, and update each tag."
folder: "Tag"
---

# Table: youtrack_tag - Query YouTrack Tags using SQL

YouTrack tags are user-owned labels that can be attached to issues and articles. Each tag carries three independent sharing configurations that decide who may see the tag, who may apply it, and who may change it, so a tag that is widely visible is not necessarily widely usable.

The `youtrack_tag` table returns the tags visible to the permanent-token user, along with the owner object, the untag-on-resolve behavior, and all three sharing settings as JSONB so no access-control detail is lost.

## Table Usage Guide

The `youtrack_tag` table provides insights into the tag vocabulary of a YouTrack instance and how that vocabulary is shared. As an administrator or workflow owner, use this table to audit which tags exist, who owns them, whether they are shared beyond their owner, and which tags automatically detach when an issue is resolved.

**Important Notes**
- Exact `id` equality uses the single-tag endpoint and accepts only database IDs, such as `6-42`.
- The `query` control column is passed verbatim to YouTrack's tag-name search. It is a tag-name search, not the issue-search syntax used elsewhere in this plugin.
- A predicate on `name` remains a local filter, because the YouTrack tag search is not documented as exact equality. Use `query` when you want the API to do the matching.
- `read_sharing_settings`, `tag_sharing_settings`, and `update_sharing_settings` are independent: visibility, use, and update access are granted separately.
- Results contain only tags visible to the current user. A specific tag is visible to its owner or to users covered by its sharing configuration.

## Examples

### Find tags by name
Use the `query` control column to let YouTrack perform the tag-name search rather than filtering locally.

```sql+postgres
select
  id,
  name,
  owner,
  untag_on_resolve
from
  youtrack_tag
where
  query = 'release'
order by
  name;
```

```sql+sqlite
select
  id,
  name,
  owner,
  untag_on_resolve
from
  youtrack_tag
where
  query = 'release'
order by
  name;
```

### Get the sharing configuration of a specific tag
Retrieve a single tag by its database ID to review who may see and apply it.

```sql+postgres
select
  name,
  read_sharing_settings,
  tag_sharing_settings
from
  youtrack_tag
where
  id = '6-42';
```

```sql+sqlite
select
  name,
  read_sharing_settings,
  tag_sharing_settings
from
  youtrack_tag
where
  id = '6-42';
```

### List tags that are removed when an issue is resolved
Surface tags with automatic untagging behavior, since they silently disappear from issues as work completes.

```sql+postgres
select
  id,
  name,
  owner ->> 'login' as owner_login
from
  youtrack_tag
where
  untag_on_resolve
order by
  name;
```

```sql+sqlite
select
  id,
  name,
  json_extract(owner, '$.login') as owner_login
from
  youtrack_tag
where
  untag_on_resolve
order by
  name;
```
