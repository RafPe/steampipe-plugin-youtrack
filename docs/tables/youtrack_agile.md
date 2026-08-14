---
title: "Steampipe Table: youtrack_agile - Query YouTrack Agile Boards using SQL"
description: "Allows users to query YouTrack agile boards, providing board names, owners, the projects each board draws from, and its sprints including the current one."
folder: "Agile Board"
---

# Table: youtrack_agile - Query YouTrack Agile Boards using SQL

A YouTrack agile board is a configured view over one or more projects, organizing their issues into sprints and swimlanes. Boards are the unit teams plan against, so knowing which projects feed a board and which sprint is currently active describes how work is actually organized in the instance.

The `youtrack_agile` table returns the agile boards available to the permanent-token user. Associated projects and sprints remain JSONB to preserve their nested representation, and `current_sprint` is exposed separately for the sprint in progress.

## Table Usage Guide

The `youtrack_agile` table provides insights into how teams have organized their work into boards and sprints. As a delivery lead or administrator, use this table to inventory boards, see which projects each board spans, spot boards without a current sprint, and find boards that have accumulated a long sprint history.

**Important Notes**
- Exact `id` equality uses the single-board endpoint and accepts only the board's database ID, such as `120-4`.
- The board collection has no documented search parameter, so predicates on `name`, `owner`, `projects`, and `sprints` are applied locally by Steampipe after the list is fetched.
- Results contain only the boards available to the token user, and reading a specific board requires permission to read that board, so results are access-dependent.
- `projects` and `sprints` may be present but empty arrays, so use array-length checks rather than null checks when looking for empty boards.
- `current_sprint` is null when the board has no sprint in progress, which is expected for boards that do not use sprints at all.

## Examples

### List all agile boards
A basic inventory of the boards visible to the token user, with their owner and active sprint.

```sql+postgres
select
  id,
  name,
  owner,
  current_sprint
from
  youtrack_agile
order by
  name;
```

```sql+sqlite
select
  id,
  name,
  owner,
  current_sprint
from
  youtrack_agile
order by
  name;
```

### List the projects associated with a board
Expand a board's `projects` array to see exactly which projects feed it.

```sql+postgres
select
  a.name,
  project.value ->> 'shortName' as project
from
  youtrack_agile as a,
  jsonb_array_elements(a.projects) as project(value)
where
  a.id = '120-4';
```

```sql+sqlite
select
  a.name,
  json_extract(p.value, '$.shortName') as project
from
  youtrack_agile as a,
  json_each(a.projects) as p
where
  a.id = '120-4';
```

### Show sprint counts per board
Compare how much sprint history each board carries, and see the name of the sprint currently in progress.

```sql+postgres
select
  name,
  jsonb_array_length(sprints) as sprint_count,
  current_sprint ->> 'name' as current_sprint_name
from
  youtrack_agile
order by
  sprint_count desc;
```

```sql+sqlite
select
  name,
  json_array_length(sprints) as sprint_count,
  json_extract(current_sprint, '$.name') as current_sprint_name
from
  youtrack_agile
order by
  sprint_count desc;
```
