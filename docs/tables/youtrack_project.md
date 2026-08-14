---
title: "Steampipe Table: youtrack_project - Query YouTrack Projects using SQL"
description: "Allows users to query YouTrack Projects, providing the project name, short name, description, and leader for every project visible to the token user."
folder: "Project"
---

# Table: youtrack_project - Query YouTrack Projects using SQL

A YouTrack project is the top-level container that groups issues, defines the custom field schema those issues use, and scopes permissions. Every project has a display name and a short name, such as `DEMO`, which forms the prefix of its human-readable issue IDs.

The `youtrack_project` table returns the projects visible to the permanent-token user. Nested objects such as `leader` are kept as JSONB, and the `raw` column preserves the complete YouTrack representation that was requested.

## Table Usage Guide

The `youtrack_project` table provides insights into the project inventory of a YouTrack instance. As an administrator or compliance engineer, use this table to enumerate projects, confirm ownership by identifying each project leader, and resolve short names to database IDs so they can be used to scope issue queries.

**Important Notes**
- Exact `id` or `short_name` equality uses `GET /api/admin/projects/{projectID}`. Both database IDs and project short names are accepted in that position.
- All other predicates are evaluated locally after the rows are fetched.
- Collection reads page through the API using `$top` and `$skip`, and they honor the SQL limit.
- Reading a specific project requires the **Read Project Basic** or **Update Project** permission. Collection results include only the projects available to the token user.

## Examples

### Basic info
List every project the token user can see, ordered by short name, together with its leader.

```sql+postgres
select
  id,
  name,
  short_name,
  leader
from
  youtrack_project
order by
  short_name;
```

```sql+sqlite
select
  id,
  name,
  short_name,
  leader
from
  youtrack_project
order by
  short_name;
```

### Get the details of a specific project
Look up a single project by its short name, which routes the request straight to the project endpoint.

```sql+postgres
select
  id,
  name,
  description
from
  youtrack_project
where
  short_name = 'DEMO';
```

```sql+sqlite
select
  id,
  name,
  description
from
  youtrack_project
where
  short_name = 'DEMO';
```

### List projects and their leader logins
Pull the leader login out of the nested JSON object to review project ownership across the instance.

```sql+postgres
select
  short_name,
  name,
  leader ->> 'login' as leader_login,
  leader ->> 'fullName' as leader_full_name
from
  youtrack_project
order by
  short_name;
```

```sql+sqlite
select
  short_name,
  name,
  json_extract(leader, '$.login') as leader_login,
  json_extract(leader, '$.fullName') as leader_full_name
from
  youtrack_project
order by
  short_name;
```
