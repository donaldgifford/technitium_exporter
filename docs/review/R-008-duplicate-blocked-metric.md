# R-008: Duplicate Blocked Query Metric

| Field    | Value                            |
| -------- | -------------------------------- |
| Severity | LOW                              |
| Category | Design                           |
| File     | `collector/collector.go:194,198` |
| Linter   | Manual review                    |

## Finding

The same value (`s.TotalBlocked`) is emitted under two different metric names:

```go
// Line 194: as part of the query type breakdown
ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalBlocked), "blocked")

// Line 198: as a standalone metric
ch <- prometheus.MustNewConstMetric(c.blockedTotal, prometheus.CounterValue, float64(s.TotalBlocked))
```

This means both of these return the same number:

- `technitium_queries_by_type_total{type="blocked"}`
- `technitium_blocked_queries_total`

## Impact

- **Storage:** Prometheus stores both series, doubling the storage for this data
  point.
- **Dashboard confusion:** Users may use either metric in queries and get the
  same result, leading to confusion about which is "correct."
- **Alerting:** An alert on one metric fires identically to an alert on the
  other.

The duplication is not harmful -- it's a convenience trade-off. Having
`technitium_blocked_queries_total` as a top-level metric makes simple queries
easier (`technitium_blocked_queries_total` vs
`technitium_queries_by_type_total{type="blocked"}`).

## Root Cause

Intentional design choice for dashboard convenience. The Technitium dashboard UI
shows blocked queries both as a standalone stat and as part of the query type
breakdown.

## Proposed Solutions

### Option A: Keep both, document the duplication (Recommended)

Add a comment in the code noting the intentional duplication. This matches how
the Technitium UI presents the data and gives dashboard authors flexibility.

**Effort:** One comment **Risk:** None

### Option B: Remove `technitium_blocked_queries_total`

Remove the standalone metric. Users would use
`technitium_queries_by_type_total{type="blocked"}` instead.

**Effort:** Low (remove descriptor, remove emit line) **Risk:** Breaking change
for any existing dashboards or alerts using `technitium_blocked_queries_total`.
Would require a version bump.

### Option C: Remove the `blocked` label from `queriesByType`

Keep only the standalone metric. The query type breakdown would cover
authoritative, recursive, cached, and dropped -- but not blocked.

**Effort:** Low **Risk:** Loses the ability to see all query types in a single
stacked graph without a separate query.

## Recommendation

**Option A.** The duplication is intentional and useful. A code comment is
sufficient.
