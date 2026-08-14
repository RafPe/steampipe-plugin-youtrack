---
title: "Steampipe Table: youtrack_article - Query YouTrack Articles using SQL"
description: "Allows users to query YouTrack knowledge base articles, providing summaries, content, parent project, reporter, tags, and creation and update timestamps."
folder: "Article"
---

# Table: youtrack_article - Query YouTrack Articles using SQL

YouTrack articles are the pages of the built-in knowledge base. Each article belongs to a project, has a reporter who created it, and can carry tags in the same way issues do, which makes the knowledge base searchable alongside the rest of the instance.

The `youtrack_article` table returns the articles visible to the permanent-token user. Nested projects, reporters, and tags remain JSONB so their full representation is preserved, and YouTrack's Unix-millisecond timestamps are converted to PostgreSQL timestamps.

## Table Usage Guide

The `youtrack_article` table provides insights into the state and freshness of your knowledge base. As a documentation owner or team lead, use this table to find stale articles, see which projects carry the most documentation, and identify articles that are missing content or ownership.

**Important Notes**
- Exact equality on either `id` or `id_readable` uses the single-article endpoint. Database IDs and readable article IDs such as `NP-A-1` are both accepted.
- The article collection documents no global filter, so predicates on `summary`, `content`, `project`, `reporter`, `tags`, `created`, and `updated` are applied locally by Steampipe after the list is fetched.
- Article visibility restrictions apply to results. A specific article is accessible to permitted users and groups, to users holding **Override Visibility Restrictions**, and to the reporter under the exception documented by YouTrack.
- `tags` is null when unavailable, but an article with no tags returns an empty array rather than null.

## Examples

### List the most recently updated articles
See what has changed in the knowledge base lately, which is the usual starting point for a documentation review.

```sql+postgres
select
  id_readable,
  summary,
  project ->> 'shortName' as project,
  updated
from
  youtrack_article
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
  youtrack_article
order by
  updated desc
limit 20;
```

### Get a specific article by its readable ID
Fetch a single article's full content using the ID shown in the YouTrack UI.

```sql+postgres
select
  id,
  id_readable,
  summary,
  content
from
  youtrack_article
where
  id_readable = 'NP-A-1';
```

```sql+sqlite
select
  id,
  id_readable,
  summary,
  content
from
  youtrack_article
where
  id_readable = 'NP-A-1';
```

### Find articles that have not been updated in a year
Stale documentation is often worse than none, so surface the articles most likely to be out of date.

```sql+postgres
select
  id_readable,
  summary,
  project ->> 'shortName' as project,
  reporter ->> 'login' as reporter_login,
  updated
from
  youtrack_article
where
  updated < now() - interval '1 year'
order by
  updated;
```

```sql+sqlite
select
  id_readable,
  summary,
  json_extract(project, '$.shortName') as project,
  json_extract(reporter, '$.login') as reporter_login,
  updated
from
  youtrack_article
where
  updated < datetime('now', '-1 year')
order by
  updated;
```
