---
title: "Steampipe Table: youtrack_issue_work_item - Query YouTrack Issue Work Items using SQL"
description: "Allows users to query YouTrack Issue Work Items, providing the time logged against issues with its author, work date, and duration."
folder: "Issue"
---

# Table: youtrack_issue_work_item - Query YouTrack Issue Work Items using SQL

A YouTrack work item is a single time-tracking entry recorded against an issue. It captures who performed the work, who created the entry, the work date, the duration, and optional descriptive text.

The `youtrack_issue_work_item` table returns visible work items from either the global work-item collection or the collection nested under one issue. Result objects are returned as JSONB, and YouTrack millisecond timestamps are converted to PostgreSQL timestamps.

## Table Usage Guide

The `youtrack_issue_work_item` table provides insights into logged effort across projects and people. As a team lead or anyone producing timesheets, use this table to build date-range reports, total minutes per issue or per person, and to reconcile reported time against planned work.

**Important Notes**
- An exact `issue_id` qualifier uses the nested issue work-item collection. Without it, `query`, range conditions on `date`, `created`, and `updated`, `author_filter`, and `creator_filter` are served by the global endpoint.
- Range operators (`=`, `>`, `>=`, `<`, `<=`) on the real timestamp columns are pushed to the global endpoint as inclusive millisecond bounds: `date` maps to `start`/`end`, `created` maps to `createdStart`/`createdEnd`, and `updated` maps to `updatedStart`/`updatedEnd`.
- An exact `id` uses the global single-resource endpoint, unless an `issue_id` qualifier is also retained.
- `query`, `author_filter`, and `creator_filter` are control columns: they exist to be supplied as qualifiers and are null unless supplied as one. `query` takes an exact YouTrack issue-search query for the global endpoint, and the two filter columns map to one or more repeated `author` and `creator` parameters.
- `author_filter` and `creator_filter` accept a database ID, a login, a `ringId`, or `me`. Use `IN` to pass repeated values.
- Do not substitute the result-bearing `author` or `creator` JSONB columns for their filter columns; only the filter columns are pushed to the API.
- A specific work item requires access to its parent issue and the **Read Work Item** permission. The global and nested collections contain only work items visible to the token user.

## Examples

### List your own logged time for a month
Produce a personal timesheet by combining `author_filter` with an inclusive date range that is pushed to the API.

```sql+postgres
select
  id,
  issue,
  author,
  date,
  duration
from
  youtrack_issue_work_item
where
  author_filter = 'me'
  and date >= timestamp '2026-08-01'
  and date <= timestamp '2026-08-31'
order by
  date desc;
```

```sql+sqlite
select
  id,
  issue,
  author,
  date,
  duration
from
  youtrack_issue_work_item
where
  author_filter = 'me'
  and date >= '2026-08-01'
  and date <= '2026-08-31'
order by
  date desc;
```

### Total minutes per issue for a set of authors
Find where a group of people spent their time, passing repeated `author` parameters with `IN`.

```sql+postgres
select
  issue ->> 'idReadable' as issue_id,
  sum((duration ->> 'minutes')::integer) as minutes
from
  youtrack_issue_work_item
where
  author_filter in ('rafal.pieniazek', '1-2')
group by
  issue ->> 'idReadable'
order by
  minutes desc;
```

```sql+sqlite
select
  json_extract(issue, '$.idReadable') as issue_id,
  sum(cast(json_extract(duration, '$.minutes') as integer)) as minutes
from
  youtrack_issue_work_item
where
  author_filter in ('rafal.pieniazek', '1-2')
group by
  json_extract(issue, '$.idReadable')
order by
  minutes desc;
```

### List the work items recorded on one issue
Supplying an exact `issue_id` switches the query to the nested issue work-item collection.

```sql+postgres
select
  id,
  author,
  date,
  duration ->> 'presentation' as spent,
  text
from
  youtrack_issue_work_item
where
  issue_id = 'DEMO-7'
order by
  date desc;
```

```sql+sqlite
select
  id,
  author,
  date,
  json_extract(duration, '$.presentation') as spent,
  text
from
  youtrack_issue_work_item
where
  issue_id = 'DEMO-7'
order by
  date desc;
```
