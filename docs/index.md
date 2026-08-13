---
organization: RafPe
category: ["software development"]
icon_url: "https://raw.githubusercontent.com/RafPe/steampipe-plugin-youtrack/main/assets/social-card.png"
brand_color: "#FF318C"
display_name: "YouTrack"
short_name: "youtrack"
description: "Steampipe plugin for querying issues, projects, users, and more from JetBrains YouTrack."
og_description: "Query YouTrack with SQL! Open source CLI. No DB required."
og_image: "/images/plugins/rafpe/youtrack-social-graphic.png"
engines: ["steampipe", "sqlite", "postgres", "export"]
---

<!-- icon_url and og_image are placeholders: the real Hub asset paths
     (/images/plugins/rafpe/youtrack.svg and -social-graphic.png) are issued
     by Turbot via the Steampipe community Slack at submission time.
     brand_color is not confirmed against JetBrains' brand guidelines. -->

# YouTrack + Steampipe

[YouTrack](https://www.jetbrains.com/youtrack/) is a project management and issue tracking tool by JetBrains.

[Steampipe](https://steampipe.io) is an open-source zero-ETL engine to instantly query cloud APIs using SQL.

This is a community-maintained plugin, not an official JetBrains product. It is not affiliated with, endorsed by, or supported by JetBrains.

List unresolved issues in a project:

```sql
select
  id_readable,
  summary,
  created
from
  youtrack_issue
where
  query = 'project: DEMO #Unresolved';
```

```
+-------------+----------------------------+---------------------------+
| id_readable | summary                    | created                   |
+-------------+----------------------------+---------------------------+
| DEMO-42     | Fix login redirect loop    | 2026-08-01T09:15:23+02:00 |
| DEMO-40     | Add SSO documentation      | 2026-07-28T14:02:11+02:00 |
+-------------+----------------------------+---------------------------+
```

## Documentation

- **[Table definitions & examples →](/plugins/rafpe/youtrack/tables)**

## Get started

### Install

Download and install the latest YouTrack plugin:

```sh
steampipe plugin install rafpe/youtrack
```

### Credentials

| Item        | Description                                                                                                                  |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Credentials | YouTrack requires a [permanent token](https://www.jetbrains.com/help/youtrack/devportal/authentication-with-permanent-token.html) and the base URL of your instance for all requests. |
| Permissions | The token inherits the permissions of the user who created it. Results only contain resources visible to that user.           |
| Radius      | Each connection represents a single YouTrack instance.                                                                        |
| Resolution  | 1. Credentials explicitly set in a Steampipe config file (`~/.steampipe/config/youtrack.spc`)<br />2. Credentials specified in the `YOUTRACK_URL` and `YOUTRACK_TOKEN` environment variables. |

### Configuration

Installing the latest YouTrack plugin will create a config file (`~/.steampipe/config/youtrack.spc`) with a single connection named `youtrack`:

```hcl
connection "youtrack" {
  plugin = "rafpe/youtrack"

  # Base URL of your YouTrack instance, without a trailing slash, e.g.
  # "https://example.youtrack.cloud" or "https://example.com/youtrack".
  # Can also be set with the YOUTRACK_URL environment variable.
  # base_url = "https://example.youtrack.cloud"

  # YouTrack permanent token. Create one in your YouTrack profile under
  # Account Security > Tokens. Can also be set with the YOUTRACK_TOKEN
  # environment variable. Keep permanent tokens out of configuration files
  # and source control.
  # token = "perm:cmFmcGU=.UGVybWFuZW50IHRva2Vu.abcdefghij1234567890"
}
```

Alternatively, you can also use the environment variables:

```sh
export YOUTRACK_URL=https://example.youtrack.cloud
export YOUTRACK_TOKEN=perm:cmFmcGU=.UGVybWFuZW50IHRva2Vu.abcdefghij1234567890
```

HTTPS is required; plain HTTP is allowed only for localhost or a loopback address. Root and sub-path installations are supported; `/api` is appended exactly once.

See [Releases](releases.md) for how versions are cut and published.
