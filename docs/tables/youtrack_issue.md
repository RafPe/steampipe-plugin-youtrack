---
title: "Steampipe Table: youtrack_issue - Query YouTrack Issues using SQL"
description: "Allows users to query YouTrack Issues, providing summaries, descriptions, reporters, timestamps, tags, and custom field values for every issue visible to the token user."
folder: "Issue"
---

# Table: youtrack_issue - Query YouTrack Issues using SQL

YouTrack is JetBrains' issue tracker, where work items are recorded as issues inside projects. Each issue carries a human-readable ID such as `DEMO-7`, a summary and description, reporter and updater references, lifecycle timestamps, tags, and a set of project-defined custom fields.

The `youtrack_issue` table returns the issues visible to the permanent-token user. Nested and polymorphic values, such as `project`, `reporter`, `tags`, and `custom_fields`, are kept as JSONB, and the `raw` column preserves the complete YouTrack representation that was requested.

## Table Usage Guide

The `youtrack_issue` table provides insights into individual work items across YouTrack projects. As a project manager, team lead, or compliance engineer, use this table to track unresolved work, audit how long issues stay open, review who reported and last touched an issue, and join issue metadata against other systems.

**Important Notes**
- Exact `id` or `id_readable` equality uses the single-issue endpoint.
- Exact `project_id` equality uses the project-scoped issue collection.
- Exact `query` equality is passed verbatim to YouTrack. The `query` column is a control column holding a YouTrack issue-search expression, which uses YouTrack search syntax and not SQL semantics.
- All other SQL predicates are evaluated locally after the rows are fetched.
- Reading a specific issue requires the **Read Issue** permission, subject to issue visibility and YouTrack's documented reporter exception. Collection results contain only issues visible to the token user.

## Examples

### Unresolved issues in a project
Push a YouTrack search expression down to the API to list the open work in a project, most recently updated first.

```sql+postgres
select
  id_readable,
  summary,
  project ->> 'shortName' as project,
  updated
from
  youtrack_issue
where
  query = 'project: DEMO State: Unresolved'
order by
  updated desc
limit 20;
```

```sql+sqlite
select
  id_readable,
  summary,
  json_extract(project, '$.shortName') as project,
  updated
from
  youtrack_issue
where
  query = 'project: DEMO State: Unresolved'
order by
  updated desc
limit 20;
```

### Get the details of a specific issue
Look up one issue by its human-readable ID, which routes the request to the single-issue endpoint instead of scanning a collection.

```sql+postgres
select
  id,
  id_readable,
  summary,
  reporter
from
  youtrack_issue
where
  id_readable = 'DEMO-7';
```

```sql+sqlite
select
  id,
  id_readable,
  summary,
  reporter
from
  youtrack_issue
where
  id_readable = 'DEMO-7';
```

### List issues by reporter login
Extract the reporter login out of the nested JSON object to see who is filing the most work in a project.

```sql+postgres
select
  reporter ->> 'login' as reporter_login,
  count(*) as issue_count
from
  youtrack_issue
where
  project_id = '0-1'
group by
  reporter ->> 'login'
order by
  issue_count desc;
```

```sql+sqlite
select
  json_extract(reporter, '$.login') as reporter_login,
  count(*) as issue_count
from
  youtrack_issue
where
  project_id = '0-1'
group by
  json_extract(reporter, '$.login')
order by
  issue_count desc;
```
