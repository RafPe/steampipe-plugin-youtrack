# Resty v3 evaluation

Date: 2026-08-10

## Decision

**Defer adoption of `resty.dev/v3`.** Re-evaluate after a stable v3.0.0 release and only if the client grows beyond its current GET/list scope.

Resty v3.0.0-rc.3 can implement every required transport behavior, but adopting it now would exchange a small, explicit `net/http` implementation for a pre-release dependency while retaining most of our YouTrack-specific policy code. It would not simplify error classification, response validation, pagination, URL-segment escaping, or the exact retry allow-list. The current client is easier to audit for the narrow workload and is already covered by focused tests.

This is a deferral, not a rejection: Resty's implementation is credible and its v3 API is a reasonable option if the client later needs broader REST features, middleware, tracing, multiple authentication schemes, or more elaborate transport configuration.

## Version and support status

The current v3 release is [`v3.0.0-rc.3`](https://github.com/go-resty/resty/releases/tag/v3.0.0-rc.3), published 2026-07-09. The official package page marks it as tagged but not stable; the module requires Go 1.23 or newer and uses the `resty.dev/v3` vanity import path ([package metadata](https://pkg.go.dev/resty.dev/v3), [tagged `go.mod`](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/go.mod)). This repository uses Go 1.26, so toolchain compatibility is not a blocker. Pre-release API/behavior churn is the principal timing concern.

## Requirement-by-requirement comparison

| Concern | Current `net/http` client | Resty v3.0.0-rc.3 | Assessment |
|---|---|---|---|
| Context cancellation | Every request uses `http.NewRequestWithContext`; retry waits select on `ctx.Done()`. | `Request.SetContext`/`WithContext` propagates context to the native request. The retry timer also selects on `Request.Context().Done()` ([request API and loop](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/request.go#L147-L178), [retry loop](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/request.go#L1452-L1577)). | Equivalent. Preserve `errors.Is(..., context.Canceled/DeadlineExceeded)` tests because Resty can combine request and transport errors. |
| Repeated query parameters | Clones `url.Values`; `Add` preserves repeated `fields` and input values. | `SetQueryParamsFromValues(url.Values)` calls `Add` for every value ([source](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/request.go#L344-L362)). | Equivalent. Avoid `SetQueryParams(map[string]string)`, which cannot represent repeats. |
| Bounded response bodies | Success bodies read `limit+1` and return `ErrResponseTooLarge`; error bodies are truncated exactly to the configured bound. | `SetResponseBodyLimit` returns `ErrResponseBodyTooLarge` when an uncompressed parsed response exceeds the bound ([API](https://pkg.go.dev/resty.dev/v3#Client.SetResponseBodyLimit)). The limit is not enforced with do-not-parse or save-to-file modes. | Adequate for normal auto-parse. Exact current error-body truncation and typed body capture would still need wrapper logic and tests. Do not enable bypass modes. |
| Body and connection cleanup | Reads then closes every response body and reports close failures. The injected `http.Client`/transport remains caller-owned. | Parsed bodies are closed by response middleware; retries drain bodies before waiting, supporting connection reuse ([auto-parse middleware](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/middleware.go#L529-L573), [retry loop](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/request.go#L1551-L1577)). `Client.Close` runs Resty-owned hooks/load-balancer/watcher cleanup but does not replace `http.Transport.CloseIdleConnections` ([source](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/client.go#L2418-L2438)). | Equivalent for ordinary requests. Ownership must remain explicit when injecting a transport; Resty silently ignores response-body close errors, unlike the current client. |
| `Retry-After` and bounded idempotent GET retry | At most three GET attempts; retries only 429, 500, 502, 503, 504; accepts delta-seconds and HTTP-date; wait is cancellable. | Retry count is bounded (`first attempt + retry count`), non-idempotent methods are excluded by default, and 429/503 `Retry-After` accepts seconds or HTTP-date ([retry API](https://pkg.go.dev/resty.dev/v3#Client.SetRetryCount), [parser/backoff](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/retry.go#L148-L246)). Conditions are configurable; built-in status helpers cover 429 and broad 5xx ([helpers](https://pkg.go.dev/resty.dev/v3#RetryConditionStatus5XX)). | Capabilities match, but defaults do **not** match our policy. Migration must disable/avoid broad defaults as needed and register the exact 429/500/502/503/504 allow-list. Set retry count to 2 for three total attempts. Note that Resty honors `Retry-After` only for 429 and 503; this matches the statuses where that header is normally relevant but is narrower than the current generic parsing. |
| Error classification | `HTTPError` classifies authentication, authorization, not-found, rate-limit, other client, transient, and server failures; bounded body and parsed `RetryAfter` are retained; successful non-JSON is a typed invalid response. | Resty exposes response status/body and supports an error result type, but it does not know these YouTrack classifications ([response API](https://pkg.go.dev/resty.dev/v3#Response)). | No simplification. Keep a domain adapter and existing error contract. Resty's execution error is not a substitute for HTTP status classification. |
| Token-safe errors and diagnostics | Errors never include request headers or token; no debug dump facility exists. | Debug logs sanitize headers containing `authorization`, `auth`, `token`, `api-key`, or `secret`, and debug bodies can be limited ([debug source](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/debug.go#L120-L194), [sanitizer](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/util.go#L286-L313), [debug body limit API](https://pkg.go.dev/resty.dev/v3#Client.SetDebugBodyLimit)). | Header redaction is good but debug remains opt-in risk: response bodies and URLs/query strings may contain sensitive data, curl generation is another diagnostic surface, and custom formatters/hooks can bypass safe presentation. Keep debug off by default, set a small body limit, prohibit tokens in URLs/query parameters, and test emitted diagnostics before enabling it. |
| Concurrent independent connections | Configuration is immutable after construction; separate clients retain separate base URLs/tokens and can run concurrently. | Client settings are guarded and Resty documents thread-safe client properties; requests are independently created with `R()`/`NewRequest()` ([client API](https://pkg.go.dev/resty.dev/v3#Client)). | Suitable if configuration is completed before use and each operation creates a fresh request. Do not share/mutate one `Request` concurrently. Prefer one Resty client per plugin connection so credentials and base URLs cannot bleed across connections. |
| `httptest` testability | `WithHTTPClient`, `httptest.Server`, and custom `RoundTripper` tests are direct and standard-library-only. | `NewWithClient(*http.Client)`, `SetTransport`, and base URL configuration preserve both `httptest.Server` and custom transport testing ([constructors](https://pkg.go.dev/resty.dev/v3#NewWithClient), [transport API](https://pkg.go.dev/resty.dev/v3#Client.SetTransport)). | Equivalent, with slightly more middleware state to account for. Port the existing behavioral tests unchanged in intent. |

## Dependency and vulnerability posture

At rc.3, Resty's tagged module has one direct non-standard-library requirement: `golang.org/x/net v0.43.0` ([official `go.mod`](https://github.com/go-resty/resty/blob/v3.0.0-rc.3/go.mod)). That is a modest isolated graph, although this plugin already has a much larger transitive graph through the Steampipe SDK, so the practical incremental cost must be measured with `go mod graph` after a future trial rather than inferred from the repository's current `go.mod` alone.

No direct `resty.dev/v3` advisory was found in the public Go Vulnerability Database during this 2026-08-10 review. However, rc.3's pinned `x/net v0.43.0` lies inside affected module-version ranges in current records, including the `x/net/html` issues fixed in v0.45.0 and v0.55.0 ([GO-2026-4440](https://pkg.go.dev/vuln/GO-2026-4440), [GO-2026-5027](https://pkg.go.dev/vuln/GO-2026-5027), [GO-2026-5030](https://pkg.go.dev/vuln/GO-2026-5030)) and an `x/net/dns/dnsmessage` issue fixed in v0.56.0 ([GO-2026-5942](https://pkg.go.dev/vuln/GO-2026-5942)). Those records name package/symbol-level affected surfaces, so module version alone does not prove that this plugin would call vulnerable code; nevertheless, the stale pin worsens scanner and maintenance posture and is another reason not to adopt rc.3. A future trial should require Resty to update the dependency (or deliberately select a patched compatible `x/net`) and run `govulncheck ./...` against the actual reachable call graph. The Go database is curated from maintainers and sources including MITRE and GitHub ([database description](https://pkg.go.dev/vuln/)).

## What Resty would and would not replace

Resty could replace request construction, authorization/header setup, body reading, retry scheduling, and basic JSON unmarshalling. It would not replace:

- base URL normalization and safe path-segment construction;
- pagination and Steampipe limit handling;
- the exact retry status allow-list;
- YouTrack-specific `HTTPError`/`InvalidResponseError` classification;
- diagnostic policy and token-leak regression tests;
- connection-scoped client construction and lifecycle ownership.

The net reduction is therefore small. The current implementation is roughly one focused transport file with no extra direct dependency, and its security-relevant behavior is visible at the call site rather than distributed across Resty configuration and middleware.

## Re-evaluation gates

Reconsider adoption when all of the following hold:

1. Resty v3.0.0 (or later) is a stable release and has had a short production-soak period.
2. A spike demonstrates fewer lines and no weaker guarantees after retaining the domain adapter.
3. Existing cancellation, repeated-param, size-bound, cleanup, retry, classification, token-leak, concurrency, and `httptest` tests pass against the adapter.
4. Debug and curl diagnostics remain disabled by default and have explicit redaction tests if exposed.
5. `go mod graph`, `go mod why -m resty.dev/v3`, and `govulncheck ./...` show an acceptable incremental supply-chain posture.

Until then, keep the current `net/http` client.
