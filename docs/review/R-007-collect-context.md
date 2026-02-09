# R-007: Hardcoded `context.Background()` in Collect

| Field    | Value                        |
| -------- | ---------------------------- |
| Severity | LOW                          |
| Category | Idioms                       |
| File     | `collector/collector.go:131` |
| Linter   | Manual review                |

## Finding

The `Collect` method creates a `context.Background()` for API calls:

```go
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
    start := time.Now()
    ctx := context.Background()
    // ...
}
```

The `prometheus.Collector` interface does not pass a context:

```go
type Collector interface {
    Describe(chan<- *Desc)
    Collect(chan<- Metric)
}
```

This means there's no way to propagate request-scoped cancellation from the
Prometheus scrape handler into the API calls.

## Impact

Minimal. The HTTP client already has a timeout configured via `ScrapeTimeout`,
so requests won't hang indefinitely. The only scenario where this matters is if
the Prometheus server cancels a scrape mid-flight (e.g., its own scrape timeout
fires) -- in that case, the exporter's HTTP goroutines would continue running
until the client timeout fires.

This is standard behavior for Prometheus exporters. The `prometheus.Collector`
interface intentionally omits context.

## Root Cause

The `prometheus.Collector` interface predates Go's context conventions. This is
a known limitation across the Prometheus ecosystem.

## Proposed Solutions

### Option A: Leave as-is (Recommended)

This is the standard pattern used by official Prometheus exporters
(`node_exporter`, `blackbox_exporter`, etc.). The client-side `ScrapeTimeout`
provides the necessary timeout behavior.

**Effort:** None **Risk:** None

### Option B: Store a base context on the Collector

```go
type Collector struct {
    client *technitium.Client
    logger *slog.Logger
    ctx    context.Context
    // ...
}
```

This is non-standard for Prometheus collectors and doesn't actually solve the
problem since you'd still need a way to update the context per-scrape.

**Effort:** Low **Risk:** Non-idiomatic, adds complexity without benefit

## Recommendation

**Option A.** No change needed. This is the correct pattern for Prometheus
collectors. The client timeout handles the actual timeout concern.
