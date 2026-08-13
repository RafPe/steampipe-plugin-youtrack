---
title: "Steampipe Table: youtrack_user - Query YouTrack Users using SQL"
description: "Allows users to query YouTrack Users, providing the login, full name, email address, and ban or online status for every account visible to the token user."
folder: "User"
---

# Table: youtrack_user - Query YouTrack Users using SQL

A YouTrack user is an account that can report, be assigned, and comment on issues. Each account has a login, a display name, an optional email address, and status flags indicating whether it is banned or currently online.

The `youtrack_user` table returns the users visible through YouTrack's current `/api/users` resource. The plugin does not use the deprecated Hub endpoints. The `raw` column preserves the complete YouTrack representation that was requested.

## Table Usage Guide

The `youtrack_user` table provides insights into the accounts registered in a YouTrack instance. As an administrator or security engineer, use this table to review the user roster, find banned accounts that still hold references in issues, and check which accounts are missing contact details.

**Important Notes**
- Exact `id` or `login` equality uses the single-user endpoint. A login is accepted anywhere YouTrack documents `userID`.
- The users collection has no documented email or name filter, so predicates on those columns are evaluated locally and may scan all visible users.
- The value `me` is treated as a literal login by this table. It is not resolved automatically to the current user.
- Basic user information needs no special permission. Reading all profile data requires the **Read User** permission, and fields hidden from the token user are returned as null.

## Examples

### Get the details of a specific user
Look up one account by login, which routes the request to the single-user endpoint instead of scanning the collection.

```sql+postgres
select
  id,
  login,
  full_name,
  email
from
  youtrack_user
where
  login = 'rafal.pieniazek';
```

```sql+sqlite
select
  id,
  login,
  full_name,
  email
from
  youtrack_user
where
  login = 'rafal.pieniazek';
```

### List users with a visible email address
Review the accounts whose email address the token user is allowed to see, along with their ban and presence status.

```sql+postgres
select
  login,
  full_name,
  banned,
  online
from
  youtrack_user
where
  email is not null
order by
  login
limit 20;
```

```sql+sqlite
select
  login,
  full_name,
  banned,
  online
from
  youtrack_user
where
  email is not null
order by
  login
limit 20;
```

### Find banned accounts
Banned accounts still appear as reporters and assignees on historical issues, so listing them helps when auditing stale ownership.

```sql+postgres
select
  login,
  full_name,
  email
from
  youtrack_user
where
  banned
order by
  login;
```

```sql+sqlite
select
  login,
  full_name,
  email
from
  youtrack_user
where
  banned = 1
order by
  login;
```
