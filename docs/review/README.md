# Code Review Findings

Review of the Go codebase for idiomatic patterns, performance, security, and
best practices.

**Automated checks:**

| Tool            | Result   |
| --------------- | -------- |
| `go vet`        | Clean    |
| `golangci-lint` | 0 issues |
| `go test -race` | All pass |

## Findings Summary

| ID                                         | Severity | File                              | Finding                                        |
| ------------------------------------------ | -------- | --------------------------------- | ---------------------------------------------- |
| [R-001](R-001-token-in-query-string.md)    | HIGH     | `pkg/technitium/client.go`        | Token passed in URL query string               |
| [R-002](R-002-errors-is-comparison.md)     | HIGH     | `cmd/technitium_exporter/main.go` | Direct error comparison instead of `errors.Is` |
| [R-003](R-003-goimports-prefix.md)         | MEDIUM   | `.golangci.yml`                   | goimports local prefix points to wrong project |
| [R-004](R-004-silent-env-parse.md)         | MEDIUM   | `config/config.go`                | Silent failure on invalid `SCRAPE_TIMEOUT`     |
| [R-005](R-005-response-size-limit.md)      | MEDIUM   | `pkg/technitium/client.go`        | No size limit on response body reads           |
| [R-006](R-006-url-construction.md)         | MEDIUM   | `pkg/technitium/client.go`        | URL built via string concatenation             |
| [R-007](R-007-collect-context.md)          | LOW      | `collector/collector.go`          | Hardcoded `context.Background()` in Collect    |
| [R-008](R-008-duplicate-blocked-metric.md) | LOW      | `collector/collector.go`          | Duplicate blocked query metric                 |
| [R-009](R-009-test-helpers.md)             | LOW      | `collector/collector_test.go`     | Test helpers don't accept `*testing.T`         |
| [R-010](R-010-missing-tests.md)            | LOW      | `config/`, `exporter/`            | No tests for config and exporter packages      |

### Counts

| Severity | Count |
| -------- | ----- |
| HIGH     | 2     |
| MEDIUM   | 4     |
| LOW      | 4     |

### Positive Patterns Observed

- `MustNewConstMetric` collector pattern per Prometheus best practices
- Concurrent API calls with `sync.WaitGroup` and `defer wg.Done()`
- Graceful degradation when settings endpoint fails
- HTTP server timeouts (ReadHeaderTimeout, ReadTimeout, WriteTimeout,
  IdleTimeout)
- Signal handling with graceful shutdown
- Consistent error wrapping with `%w`
- Table-driven tests in client package
- `httptest.Server`-based testing (no interface mocking)
- Response body close on all paths including error path
- Sentinel errors in `config/errors.go`
