# Exporter Enhancements Plan: New Metrics from Existing API

## Context

The Technitium `/api/dashboard/stats/get` endpoint returns significantly more
data than we currently parse. The `response` object contains these top-level
keys:

| Key                      | Currently Parsed | Contains                                                                           |
| ------------------------ | ---------------- | ---------------------------------------------------------------------------------- |
| `stats`                  | Yes              | Aggregate counters (queries, responses, zones, etc.)                               |
| `queryTypeChartData`     | **No**           | Query counts by DNS record type (A, AAAA, TXT, HTTPS, PTR, SRV, SOA, SVCB)         |
| `protocolTypeChartData`  | **No**           | Query counts by transport protocol (UDP, TCP)                                      |
| `topClients`             | **No**           | List of top clients with hit counts and rate-limit status                          |
| `topDomains`             | **No**           | List of top queried domains with hit counts                                        |
| `topBlockedDomains`      | **No**           | List of top blocked domains with hit counts                                        |
| `queryResponseChartData` | **No**           | Authoritative/Recursive/Cached/Blocked/Dropped (duplicate of stats, skip)          |
| `mainChartData`          | **No**           | Per-minute time-series buckets (Prometheus handles this via scrape interval, skip) |

This plan adds **6 new metric families** (5 from stats response chart data, 1
from settings uptime). No new API calls are needed.

## New Metrics

### 1. `technitium_queries_by_record_type_total` (Counter, label: `record_type`)

**Source:** `response.queryTypeChartData`

```json
{
  "queryTypeChartData": {
    "A": 4023,
    "AAAA": 2577,
    "TXT": 798,
    "HTTPS": 490,
    "PTR": 344,
    "SRV": 12,
    "SOA": 8,
    "SVCB": 3
  }
}
```

**Metric output:**

```text
technitium_queries_by_record_type_total{record_type="A"} 4023
technitium_queries_by_record_type_total{record_type="AAAA"} 2577
technitium_queries_by_record_type_total{record_type="TXT"} 798
...
```

**Notes:**

- Keys are dynamic -- iterate over the map rather than hardcoding types
- Uses `record_type` label to avoid collision with existing
  `technitium_queries_by_type_total{type=...}` which tracks resolution type
  (authoritative, recursive, cached, blocked, dropped)
- Maps to Unbound's `unbound_query_types_count{type="A"}` panel (piechart)

### 2. `technitium_queries_by_protocol_total` (Counter, label: `protocol`)

**Source:** `response.protocolTypeChartData`

```json
{
  "protocolTypeChartData": {
    "UDP": 8336,
    "TCP": 317
  }
}
```

**Metric output:**

```text
technitium_queries_by_protocol_total{protocol="udp"} 8336
technitium_queries_by_protocol_total{protocol="tcp"} 317
```

**Notes:**

- Lowercase the protocol label value for consistency
- Maps to Unbound's `unbound_query_udpout_count` / `unbound_query_tcpout_count`
  stat panel
- Keys may also include `TLS`, `HTTPS`, `QUIC` depending on server config --
  iterate dynamically

### 3. `technitium_top_clients` (Gauge, labels: `client`, `rate_limited`)

**Source:** `response.topClients`

```json
{
  "topClients": [
    { "name": "10.10.11.18", "hits": 2298, "rateLimited": false },
    { "name": "10.10.11.1", "hits": 1456, "rateLimited": false },
    { "name": "10.10.10.50", "hits": 892, "rateLimited": true }
  ]
}
```

**Metric output:**

```text
technitium_top_clients{client="10.10.11.18", rate_limited="false"} 2298
technitium_top_clients{client="10.10.11.1", rate_limited="false"} 1456
technitium_top_clients{client="10.10.10.50", rate_limited="true"} 892
```

**Notes:**

- Gauge because the list represents a point-in-time snapshot of top N clients
- The API returns a variable-length list (typically top 10)
- `rate_limited` label provides visibility into rate-limiting behavior
- Maps to Unbound's Loki-based "Top Client Queries" table, but here we get it
  from the API directly
- **Cardinality consideration:** Bounded by Technitium's top-N limit (default
  10), so label cardinality is safe

### 4. `technitium_top_domains` (Gauge, label: `domain`)

**Source:** `response.topDomains`

```json
{
  "topDomains": [
    { "name": "api.github.com", "hits": 136 },
    { "name": "dns.google", "hits": 98 }
  ]
}
```

**Metric output:**

```text
technitium_top_domains{domain="api.github.com"} 136
technitium_top_domains{domain="dns.google"} 98
```

**Notes:**

- Gauge (point-in-time snapshot)
- Bounded cardinality (top-N from API)
- Useful for "most queried domains" table panels in Grafana

### 5. `technitium_top_blocked_domains` (Gauge, label: `domain`)

**Source:** `response.topBlockedDomains`

```json
{
  "topBlockedDomains": [
    { "name": "stats.grafana.org", "hits": 1426 },
    { "name": "telemetry.example.com", "hits": 312 }
  ]
}
```

**Metric output:**

```text
technitium_top_blocked_domains{domain="stats.grafana.org"} 1426
technitium_top_blocked_domains{domain="telemetry.example.com"} 312
```

**Notes:**

- Gauge (point-in-time snapshot)
- Maps to Unbound's Loki-based "Top Blocked Domains" table
- High-value panel for identifying what the blocklist is catching

## Fields We Skip

| Field                    | Why Skip                                                                                                                           |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| `queryResponseChartData` | Duplicates `stats.totalAuthoritative`, `stats.totalRecursive`, etc. already exposed as `technitium_queries_by_type_total`          |
| `mainChartData`          | Per-minute time-series buckets; Prometheus handles time-series natively via scrape interval -- exposing this would be anti-pattern |

## Files to Modify

| File                                             | Change                                                                             |
| ------------------------------------------------ | ---------------------------------------------------------------------------------- |
| `pkg/technitium/types.go`                        | Add chart data fields to `StatsResponseData`, add `TopClient` and `TopEntry` types |
| `collector/collector.go`                         | Add 6 metric descriptors, `topEntriesEnabled` flag, collection logic               |
| `cmd/technitium_exporter/main.go`                | Add `--collector.top-entries` flag, pass to collector                              |
| `collector/collector_test.go`                    | Add new fields to mock JSON, verify new metrics                                    |
| `pkg/technitium/client_test.go`                  | Verify new fields parse correctly                                                  |
| `contrib/grafana/technitium-dark-dashboard.json` | Add new panels for query types, protocol, top entries, uptime                      |
| `CLAUDE.md`                                      | Update metrics table                                                               |

### 1. `pkg/technitium/types.go`

Add new types and expand `StatsResponseData`:

```go
// StatsResponseData contains the stats data from the dashboard endpoint.
type StatsResponseData struct {
    Stats                  Stats              `json:"stats"`
    QueryTypeChartData     map[string]int64   `json:"queryTypeChartData"`
    ProtocolTypeChartData  map[string]int64   `json:"protocolTypeChartData"`
    TopClients             []TopClient        `json:"topClients"`
    TopDomains             []TopEntry         `json:"topDomains"`
    TopBlockedDomains      []TopEntry         `json:"topBlockedDomains"`
}

// TopClient represents a top client entry from the dashboard API.
type TopClient struct {
    Name        string `json:"name"`
    Hits        int64  `json:"hits"`
    RateLimited bool   `json:"rateLimited"`
}

// TopEntry represents a top domain entry from the dashboard API.
type TopEntry struct {
    Name string `json:"name"`
    Hits int64  `json:"hits"`
}
```

**Design decisions:**

- `QueryTypeChartData` and `ProtocolTypeChartData` use `map[string]int64`
  because keys are dynamic
- `TopClient` is separate from `TopEntry` because it has the `rateLimited` field
- `TopEntry` is reused for both `topDomains` and `topBlockedDomains`

### 2. `collector/collector.go`

Add 5 new metric descriptors to the `Collector` struct and `NewCollector()`:

```go
// New fields in Collector struct:
queriesByRecordType  *prometheus.Desc
queriesByProtocol    *prometheus.Desc
topClients           *prometheus.Desc
topDomains           *prometheus.Desc
topBlockedDomains    *prometheus.Desc
```

Descriptor definitions:

```go
queriesByRecordType: prometheus.NewDesc(
    prometheus.BuildFQName(namespace, "queries_by_record_type", "total"),
    "DNS queries by record type (A, AAAA, TXT, etc.).",
    []string{"record_type"}, nil,
),
queriesByProtocol: prometheus.NewDesc(
    prometheus.BuildFQName(namespace, "queries_by_protocol", "total"),
    "DNS queries by transport protocol.",
    []string{"protocol"}, nil,
),
topClients: prometheus.NewDesc(
    prometheus.BuildFQName(namespace, "top_clients", "hits"),
    "Top clients by query count.",
    []string{"client", "rate_limited"}, nil,
),
topDomains: prometheus.NewDesc(
    prometheus.BuildFQName(namespace, "top_domains", "hits"),
    "Top queried domains by hit count.",
    []string{"domain"}, nil,
),
topBlockedDomains: prometheus.NewDesc(
    prometheus.BuildFQName(namespace, "top_blocked_domains", "hits"),
    "Top blocked domains by hit count.",
    []string{"domain"}, nil,
),
```

Add to `Describe()` and extend `Collect()` with:

```go
// Query type chart data (A, AAAA, TXT, etc.)
for recordType, count := range stats.Response.QueryTypeChartData {
    ch <- prometheus.MustNewConstMetric(
        c.queriesByRecordType, prometheus.CounterValue,
        float64(count), recordType,
    )
}

// Protocol type chart data (UDP, TCP, etc.)
for protocol, count := range stats.Response.ProtocolTypeChartData {
    ch <- prometheus.MustNewConstMetric(
        c.queriesByProtocol, prometheus.CounterValue,
        float64(count), strings.ToLower(protocol),
    )
}

// Top clients
for _, client := range stats.Response.TopClients {
    ch <- prometheus.MustNewConstMetric(
        c.topClients, prometheus.GaugeValue,
        float64(client.Hits), client.Name, strconv.FormatBool(client.RateLimited),
    )
}

// Top domains
for _, domain := range stats.Response.TopDomains {
    ch <- prometheus.MustNewConstMetric(
        c.topDomains, prometheus.GaugeValue,
        float64(domain.Hits), domain.Name,
    )
}

// Top blocked domains
for _, domain := range stats.Response.TopBlockedDomains {
    ch <- prometheus.MustNewConstMetric(
        c.topBlockedDomains, prometheus.GaugeValue,
        float64(domain.Hits), domain.Name,
    )
}
```

### 3. `pkg/technitium/client.go`

No changes needed -- `GetStats()` already reads the full response body and
unmarshals into `StatsResponse`. Adding fields to `StatsResponseData` is
sufficient; `json.Unmarshal` will populate them automatically.

### 4. Tests

Update test fixtures in:

- `collector/collector_test.go` -- Add new fields to mock JSON responses, verify
  new metrics appear
- `pkg/technitium/client_test.go` -- Verify new fields are parsed correctly from
  mock responses

Test JSON fixture should include:

```json
{
  "status": "ok",
  "response": {
    "stats": { ... },
    "queryTypeChartData": {"A": 100, "AAAA": 50, "TXT": 10},
    "protocolTypeChartData": {"UDP": 140, "TCP": 20},
    "topClients": [
      {"name": "10.0.0.1", "hits": 80, "rateLimited": false}
    ],
    "topDomains": [
      {"name": "example.com", "hits": 60}
    ],
    "topBlockedDomains": [
      {"name": "ads.example.com", "hits": 30}
    ]
  }
}
```

### 5. `contrib/grafana/technitium-dark-dashboard.json`

Add new panels:

- **Query Types** piechart (donut) using
  `technitium_queries_by_record_type_total`
- **Protocol Distribution** stat panel using
  `technitium_queries_by_protocol_total`
- **Top Clients** table using `technitium_top_clients_hits`
- **Top Domains** table using `technitium_top_domains_hits`
- **Top Blocked Domains** table using `technitium_top_blocked_domains_hits`

### 6. Documentation

Update `CLAUDE.md` metrics table with new metrics.

## Metric Naming Review

| Metric                                    | Follows Prometheus naming conventions?                    |
| ----------------------------------------- | --------------------------------------------------------- |
| `technitium_queries_by_record_type_total` | Yes -- `_total` suffix for counter, descriptive subsystem |
| `technitium_queries_by_protocol_total`    | Yes -- same pattern                                       |
| `technitium_top_clients_hits`             | Yes -- gauge, `_hits` describes the value                 |
| `technitium_top_domains_hits`             | Yes -- same pattern                                       |
| `technitium_top_blocked_domains_hits`     | Yes -- same pattern                                       |
| `technitium_server_uptime_seconds`        | Yes -- gauge, `_seconds` suffix for duration              |

## Decisions

1. **`record_type` label values stay uppercase.** The API returns uppercase (A,
   AAAA, TXT) and DNS record types are conventionally uppercase. No
   transformation needed.

2. **Top entries enabled by default, disabled via flag.** Add a
   `--collector.top-entries` flag (default: `true`). When disabled, the three
   top-N metrics (`top_clients`, `top_domains`, `top_blocked_domains`) are not
   registered or collected. This lets users who consider client IPs or domain
   names sensitive opt out.

3. **Add `technitium_server_uptime_seconds`.** Parse the `uptimestamp` field
   from the settings endpoint (already fetched but unused) and expose as a
   gauge. This is independent of the chart data work but low-effort and useful
   for the dashboard status row.

## Additional Changes for Decision 2

### `cmd/technitium_exporter/main.go`

Add flag:

```go
topEntries = kingpin.Flag(
    "collector.top-entries",
    "Enable top clients/domains/blocked domains metrics.",
).Default("true").Bool()
```

Pass to collector constructor:

```go
collector.NewCollector(client, logger, *topEntries)
```

### `collector/collector.go`

Update `Collector` struct and `NewCollector` signature:

```go
type Collector struct {
    // ...existing fields...
    topEntriesEnabled bool
}

func NewCollector(client *technitium.Client, logger *slog.Logger, topEntries bool) *Collector {
    c := &Collector{
        // ...existing...
        topEntriesEnabled: topEntries,
    }
    if topEntries {
        c.topClients = prometheus.NewDesc(...)
        c.topDomains = prometheus.NewDesc(...)
        c.topBlockedDomains = prometheus.NewDesc(...)
    }
    return c
}
```

Guard in `Describe()` and `Collect()`:

```go
if c.topEntriesEnabled {
    ch <- c.topClients
    ch <- c.topDomains
    ch <- c.topBlockedDomains
}
```

## Additional Changes for Decision 3

### New Metric: `technitium_server_uptime_seconds`

**Source:** `settings.response.uptimestamp` (ISO 8601 string representing server
start time)

Add descriptor:

```go
uptimeSeconds: prometheus.NewDesc(
    prometheus.BuildFQName(namespace, "server", "uptime_seconds"),
    "Technitium DNS server uptime in seconds.",
    nil, nil,
),
```

In `Collect()`, parse the timestamp and compute uptime:

```go
if settingsErr == nil {
    startTime, err := time.Parse(time.RFC3339, settings.Response.Uptimestamp)
    if err == nil {
        uptime := time.Since(startTime).Seconds()
        ch <- prometheus.MustNewConstMetric(c.uptimeSeconds, prometheus.GaugeValue, uptime)
    }
}
```

**Note:** The uptime metric is only emitted when the settings endpoint succeeds
(requires admin token). This is consistent with how `serverInfo` already handles
the optional settings endpoint.

## Verification

1. `go build ./...` -- compiles
2. `make test` -- all tests pass including new test cases
3. `make lint` -- no new lint issues
4. Scrape the exporter and verify new metrics appear with expected labels
5. Import updated dashboard and verify new panels render
