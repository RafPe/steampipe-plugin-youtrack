---
title: "Steampipe Table: youtrack_issue_comment - Query YouTrack Issue Comments using SQL"
description: "Allows users to query YouTrack Issue Comments, providing the text, author, and timestamps of the discussion recorded on a single issue."
folder: "Issue"
---

# Table: youtrack_issue_comment - Query YouTrack Issue Comments using SQL

YouTrack issue comments are the threaded discussion attached to an issue. Each comment carries its text, the user who wrote it, a link back to the parent issue, and creation and update timestamps.

The `youtrack_issue_comment` table returns the comments of one accessible issue at a time. YouTrack publishes no global comments collection, so a parent issue must always be named in the query.

## Table Usage Guide

The `youtrack_issue_comment` table provides insights into the conversation history of an individual issue. As a project manager or support engineer, use this table to reconstruct how an issue was triaged, to find who last responded, or to feed comment text into reporting alongside the issue data itself.

**Important Notes**
- `issue_id` is required on every query. YouTrack exposes no global comments collection, so a query without an exact `issue_id` cannot be answered and a bare `select * from youtrack_issue_comment` will fail. The `issue_id` column echoes back the value supplied in the qualifier.
- An exact `issue_id` selects `/api/issues/{issueID}/comments`. It accepts any parent issue identifier YouTrack understands, including a database ID such as `2-15` or a readable issue ID such as `DEMO-7`.
- Fetching a single comment requires both `issue_id` and the comment's database `id`.
- Predicates on `text`, `author`, `created`, and `updated` are evaluated locally after the comments are fetched, so they do not reduce the work done against the API.
- Reading comments requires access to the parent issue. An author can always see their own comment, while comment visibility settings can hide a comment from other readers.

## Examples

### List all comments on an issue
Reconstruct the discussion on a single issue in chronological order.

```sql+postgres
select
  id,
  text,
  author,
  created
from
  youtrack_issue_comment
where
  issue_id = 'DEMO-7'
order by
  created;
```

```sql+sqlite
select
  id,
  text,
  author,
  created
from
  youtrack_issue_comment
where
  issue_id = 'DEMO-7'
order by
  created;
```

### Get a specific comment
Retrieve one comment when you already know its database ID, for example from a webhook payload or an audit trail.

```sql+postgres
select
  id,
  issue_id,
  text,
  updated
from
  youtrack_issue_comment
where
  issue_id = 'DEMO-7'
  and id = '4-42';
```

```sql+sqlite
select
  id,
  issue_id,
  text,
  updated
from
  youtrack_issue_comment
where
  issue_id = 'DEMO-7'
  and id = '4-42';
```

### Count comments per author on an issue
Show who is driving the conversation on an issue. The author predicate is applied locally, so all comments on the issue are fetched first.

```sql+postgres
select
  author ->> 'login' as author_login,
  count(*) as comment_count
from
  youtrack_issue_comment
where
  issue_id = 'DEMO-7'
group by
  author ->> 'login'
order by
  comment_count desc;
```

```sql+sqlite
select
  json_extract(author, '$.login') as author_login,
  count(*) as comment_count
from
  youtrack_issue_comment
where
  issue_id = 'DEMO-7'
group by
  json_extract(author, '$.login')
order by
  comment_count desc;
```
