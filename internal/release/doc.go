// Package release implements deterministic PR release-metadata validation
// and next-version computation for steampipe-youtrack's review-gated release
// process. It never performs network I/O: callers gather GitHub data with
// `gh api` and feed a bounded JSON document to the releasectl CLI, which is
// a thin wrapper around this package's Run function.
package release
