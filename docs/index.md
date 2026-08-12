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
---

<!-- icon_url and og_image are placeholders: the real Hub asset paths
     (/images/plugins/rafpe/youtrack.svg and -social-graphic.png) are issued
     by Turbot via the Steampipe community Slack at submission time.
     brand_color is not confirmed against JetBrains' brand guidelines. -->

# YouTrack plugin

The YouTrack plugin exposes read-only REST API resources as Steampipe tables. Configure `base_url` and a permanent `token`; every request explicitly selects fields, respects cancellation and SQL limits, and never logs authorization headers.

This is a community-maintained plugin, not an official JetBrains product. It
is not affiliated with, endorsed by, or supported by JetBrains.

See the table pages below and the repository README for installation and development commands. See [Releases](releases.md) for how versions are cut and published.
