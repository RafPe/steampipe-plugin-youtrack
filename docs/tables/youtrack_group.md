---
title: "Steampipe Table: youtrack_group - Query YouTrack User Groups using SQL"
description: "Allows users to query YouTrack user groups, providing the group name, description, and the visible members of each group."
folder: "Group"
---

# Table: youtrack_group - Query YouTrack User Groups using SQL

A YouTrack user group collects accounts so that roles and permissions can be granted to many people at once. Each group has a name, an optional description, a membership list, and a **Visible to** setting that controls who can see it.

The `youtrack_group` table returns the groups visible through YouTrack's current `/api/groups` resource. The `users` column keeps the visible membership as a JSONB array, and the `raw` column preserves the complete YouTrack representation that was requested.

## Table Usage Guide

The `youtrack_group` table provides insights into how access is organized in a YouTrack instance. As a security or compliance engineer, use this table to review group membership, find groups that have grown unexpectedly large, and confirm that the right accounts hold the roles attached to each group.

**Important Notes**
- Exact `id` equality uses the single-group endpoint, and only database IDs are accepted there. A group short name or display name will not work in that position.
- Exact `query` equality is passed verbatim to the collection endpoint. The `query` column is a control column holding the group query sent to YouTrack.
- Equality on `name` is not an identifier lookup, so it is evaluated locally rather than pushed to the API.
- Listing groups requires the **Read Groups**, **Update Project**, or **Low-Level Admin Read** permission. A group's **Visible to** setting can further restrict both which groups are returned and which membership data is populated.

## Examples

### Search for groups by name
Push a group query down to the collection endpoint to find the groups matching a term.

```sql+postgres
select
  id,
  name,
  description
from
  youtrack_group
where
  query = 'developers'
order by
  name;
```

```sql+sqlite
select
  id,
  name,
  description
from
  youtrack_group
where
  query = 'developers'
order by
  name;
```

### List the members of a specific group
Expand the JSON membership array to see the individual logins in one group, looked up by its database ID.

```sql+postgres
select
  g.name,
  member.value ->> 'login' as member_login
from
  youtrack_group as g,
  jsonb_array_elements(g.users) as member
where
  g.id = '1-2';
```

```sql+sqlite
select
  g.name,
  json_extract(member.value, '$.login') as member_login
from
  youtrack_group as g,
  json_each(g.users) as member
where
  g.id = '1-2';
```

### Find the largest groups
Groups with a broad membership grant their permissions widely, so they deserve the closest review.

```sql+postgres
select
  name,
  jsonb_array_length(users) as member_count
from
  youtrack_group
where
  users is not null
order by
  member_count desc
limit 10;
```

```sql+sqlite
select
  name,
  json_array_length(users) as member_count
from
  youtrack_group
where
  users is not null
order by
  member_count desc
limit 10;
```
