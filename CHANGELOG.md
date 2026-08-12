# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [0.1.1] - 2026-08-12

### Changed

- Renamed the repository from steampipe-youtrack to steampipe-plugin-youtrack to satisfy the Steampipe Hub naming convention; module path, install/documentation URLs, and workflow references were updated to match.

[0.1.1]: https://github.com/RafPe/steampipe-plugin-youtrack/compare/v0.1.0...v0.1.1


## [0.1.0] - 2026-08-11

### Added

- Initial read-only Steampipe plugin for JetBrains YouTrack.
- Tables for projects, issues, users, groups, tags, saved queries, articles, agile boards, issue comments, and issue work items.
- Server-side qualifier pushdown, pagination, query-limit handling, context cancellation, bounded retries, and classified API errors.
- Unit, contract, integration, race, coverage, lint, vulnerability, and containerized end-to-end test workflows.
- Documentation, query cookbook, local development commands, and branded repository artwork.

### Dependencies

- Upgrade klauspost/compress to 1.18.7 to resolve GO-2026-5841 in the transitive dependency graph.

[0.1.0]: https://github.com/RafPe/steampipe-youtrack/compare/v0.0.0...v0.1.0

