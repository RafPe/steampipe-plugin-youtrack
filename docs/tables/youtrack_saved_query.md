---
title: "Steampipe Table: youtrack_saved_query - Query YouTrack Saved Queries using SQL"
description: "Allows users to query YouTrack Saved Queries, providing the stored issue-search expressions, their owners, and the settings that control who can read and update them."
folder: "Saved Query"
---

# Table: youtrack_saved_query - Query YouTrack Saved Queries using SQL

A YouTrack saved query is a named, reusable issue search that a user stores and optionally shares with other users and groups. Saved queries are how teams standardize on a shared definition of concepts such as "open bugs in this release" or "everything assigned to me this sprint".

The `youtrack_saved_query` table returns the saved searches visible to the permanent-token user, including the stored search expression in `query_text` and the owner and sharing settings as JSONB so no access-control information is lost through flattening.

## Table Usage Guide

The `youtrack_saved_query` table provides insights into the shared search vocabulary of a YouTrack instance. As a team lead or administrator, use this table to inventory saved searches, see which search expressions your organization relies on, and identify searches that are shared broadly versus those that remain private to their author.

**Important Notes**
- Exact `id` equality uses the single-resource endpoint and accepts the saved query's database ID, such as `10-42`.
- The saved-query collection has no documented search parameter, so predicates on `name`, `query_text`, `owner`, and the sharing settings are applied locally by Steampipe after the full list is fetched.
- Results contain only the saved searches visible to the current user. A specific saved search is readable by its author or by members of the groups it is shared with.
- `read_sharing_settings` and `update_sharing_settings` are separate: being able to see a saved query does not imply being able to change it.

## Examples

### List all saved queries
A quick inventory of every saved search visible to the token user and the expression behind it.

```sql+postgres
select
  id,
  name,
  query_text,
  owner
from
  youtrack_saved_query
order by
  name;
```

```sql+sqlite
select
  id,
  name,
  query_text,
  owner
from
  youtrack_saved_query
order by
  name;
```

### Get the sharing configuration of a specific saved query
Retrieve one saved query by its database ID to review its expression and who can read it.

```sql+postgres
select
  name,
  query_text,
  read_sharing_settings
from
  youtrack_saved_query
where
  id = '10-42';
```

```sql+sqlite
select
  name,
  query_text,
  read_sharing_settings
from
  youtrack_saved_query
where
  id = '10-42';
```

### Find saved queries that reference a specific project
Locate the stored searches that scope to a given project, which is useful before archiving or renaming that project.

```sql+postgres
select
  id,
  name,
  query_text,
  owner ->> 'login' as owner_login
from
  youtrack_saved_query
where
  query_text ilike '%project: DEMO%'
order by
  name;
```

```sql+sqlite
select
  id,
  name,
  query_text,
  json_extract(owner, '$.login') as owner_login
from
  youtrack_saved_query
where
  query_text like '%project: DEMO%'
order by
  name;
```
