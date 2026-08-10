# Query cookbook

These examples assume the connection is named `youtrack`. Predicates on `query`, exact identifiers, project-scoped issues, and issue-scoped work items are sent to YouTrack; other SQL expressions are evaluated by Steampipe.

## 1. Recently updated unresolved issues reported by a user

```sql
select id, id_readable, summary, project ->> 'shortName' as project, updated
from youtrack.youtrack_issue
where query = 'reporter: rafal.pieniazek State: Unresolved'
order by updated desc
limit 100;
```

## 2. Fetch one issue by readable ID

```sql
select id, id_readable, summary, description, reporter, custom_fields
from youtrack.youtrack_issue
where id_readable = 'DEMO-42';
```

## 3. Count unresolved issues by project

```sql
select
  project ->> 'shortName' as project,
  count(*) as unresolved_issues
from youtrack.youtrack_issue
where query = 'State: Unresolved'
group by project ->> 'shortName'
order by unresolved_issues desc;
```

## 4. Join issues to projects

```sql
select
  i.id_readable,
  i.summary,
  p.short_name,
  p.name as project_name,
  i.updated
from youtrack.youtrack_project p
join youtrack.youtrack_issue i on i.project_id = p.id
where i.query = 'State: Unresolved'
order by i.updated desc
limit 100;
```

## 5. Issue age and time since last update

```sql
select
  id_readable,
  summary,
  current_timestamp - created as age,
  current_timestamp - updated as idle_for
from youtrack.youtrack_issue
where query = 'State: Unresolved'
order by idle_for desc
limit 50;
```

## 6. Unresolved issues with no comments

```sql
select id_readable, summary, reporter, created
from youtrack.youtrack_issue
where query = 'State: Unresolved'
  and comments_count = 0
order by created;
```

## 7. Issues grouped by reporter

```sql
select
  coalesce(reporter ->> 'fullName', reporter ->> 'login', '<unknown>') as reporter,
  count(*) as issue_count
from youtrack.youtrack_issue
where query = 'State: Unresolved'
group by 1
order by issue_count desc, reporter;
```

## 8. Comments for a specific issue with author details

```sql
select
  c.id,
  c.author ->> 'fullName' as author,
  c.created,
  c.text
from youtrack.youtrack_issue_comment c
where c.issue_id = 'DEMO-42'
order by c.created;
```

## 9. Issues joined with their comments

```sql
with selected_issues as (
  select id_readable, summary
  from youtrack.youtrack_issue
  where query = 'reporter: rafal.pieniazek State: Unresolved'
  limit 20
)
select
  i.id_readable,
  i.summary,
  c.author ->> 'fullName' as comment_author,
  c.created as commented_at,
  c.text
from selected_issues i
left join youtrack.youtrack_issue_comment c on c.issue_id = i.id_readable
order by i.id_readable, c.created;
```

## 10. Work logged by the current user in a date range

```sql
select
  id,
  issue ->> 'idReadable' as issue_id,
  author ->> 'fullName' as author,
  date,
  duration ->> 'presentation' as duration
from youtrack.youtrack_issue_work_item
where author_filter = 'me'
  and start_date = timestamp '2026-08-01'
  and end_date = timestamp '2026-08-10'
order by date desc
limit 100;
```

## 11. Total work minutes per issue

```sql
select
  issue ->> 'idReadable' as issue_id,
  sum((duration ->> 'minutes')::integer) as minutes,
  round(sum((duration ->> 'minutes')::numeric) / 60, 2) as hours
from youtrack.youtrack_issue_work_item
where author_filter = 'me'
  and start_date = timestamp '2026-08-01'
  and end_date = timestamp '2026-08-31'
group by issue ->> 'idReadable'
order by minutes desc;
```

## 12. Join work items to issue summaries

```sql
with logged_work as (
  select
    issue ->> 'idReadable' as issue_id,
    date,
    (duration ->> 'minutes')::integer as minutes
  from youtrack.youtrack_issue_work_item
  where author_filter = 'me'
    and start_date = timestamp '2026-08-01'
    and end_date = timestamp '2026-08-31'
)
select
  w.issue_id,
  i.summary,
  sum(w.minutes) as minutes
from logged_work w
join youtrack.youtrack_issue i on i.id_readable = w.issue_id
group by w.issue_id, i.summary
order by minutes desc;
```

## 13. Search groups and inspect membership JSON

```sql
select id, name, description, raw -> 'users' as users
from youtrack.youtrack_group
where query = 'data'
order by name;
```

## 14. Knowledge-base articles joined to projects

```sql
select
  a.id_readable,
  a.summary,
  p.short_name,
  p.name as project_name
from youtrack.youtrack_article a
left join youtrack.youtrack_project p
  on a.project ->> 'id' = p.id
order by p.short_name, a.id_readable;
```

## 15. Project activity dashboard

```sql
with issue_stats as (
  select
    project_id,
    count(*) as total,
    count(*) filter (where resolved is null) as unresolved,
    max(updated) as last_issue_update
  from youtrack.youtrack_issue
  where query = 'has: project'
  group by project_id
)
select
  p.short_name,
  p.name,
  coalesce(s.total, 0) as visible_issues,
  coalesce(s.unresolved, 0) as unresolved_issues,
  s.last_issue_update
from youtrack.youtrack_project p
left join issue_stats s on s.project_id = p.id
order by unresolved_issues desc, p.short_name;
```

## Pushdown note

Prefer `query`, `id`, `id_readable`, `login`, `short_name`, `project_id`, and `issue_id` where applicable. For example, an email predicate on `youtrack_user` is evaluated locally because YouTrack does not document an equivalent users-collection email filter.
