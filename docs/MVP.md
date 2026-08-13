# MVP Implementation Guide - technitium_exporter v0.0.1

## Overview

This document outlines the implementation plan for a Prometheus exporter for
Technitium DNS Server, following
[Prometheus exporter best practices](https://prometheus.io/docs/instrumenting/writing_exporters/).

---

## Design Decisions

### Collector Pattern (Critical)

Per Prometheus best practices: **Never update metrics on each scrape using
direct instrumentation. Create new metrics each time using
`MustNewConstMetric`.**

This prevents:

- Race conditions between scrapes
- Orphaned label values from deleted series
- Stale metric values

### Metric Naming Conventions

Following Prometheus naming standards:

- Prefix all metrics with `technitium_`
- Use `snake_case` (convert any camelCase from API)
- Use base units: `seconds` not `milliseconds`, `bytes` not `kilobytes`
- Suffix counters with `_total`
- Never include label names in metric names

### Single Instance Per Exporter

Each exporter instance monitors exactly one Technitium server. Multi-instance
monitoring is handled at the Prometheus scrape config level, not in the
exporter.

---

## Technitium API Endpoints (MVP)

### Primary: `/api/dashboard/stats/get`

**Request:**

```http
GET /api/dashboard/stats/get?token=<token>&type=LastHour
```

**Key Response Fields:**

```json
{
  "response": {
    "stats": {
      "totalQueries": 123456,
      "totalNoError": 120000,
      "totalServerFailure": 100,
      "totalNxDomain": 3000,
      "totalRefused": 50,
      "totalAuthoritative": 5000,
      "totalRecursive": 118000,
      "totalCached": 80000,
      "totalBlocked": 10000,
      "totalDropped": 5,
      "totalClients": 25,
      "zones": 5,
      "cachedEntries": 5000,
      "allowedZones": 0,
      "blockedZones": 3,
      "blockListZones": 150000
    }
  }
}
```

### Secondary: `/api/settings/get`

**Request:**

```http
GET /api/settings/get?token=<token>
```

**Key Response Fields:**

```json
{
  "response": {
    "version": "13.0",
    "uptimestamp": "2024-01-15T10:30:00Z",
    "dnsServerDomain": "dns.example.com"
  }
}
```

---

## Metrics Specification

### Info Metric (Gauge, value=1)

```prometheus
# HELP technitium_server_info Technitium DNS server information
# TYPE technitium_server_info gauge
technitium_server_info{version="13.0",server_domain="dns.example.com"} 1
```

### Query Metrics (Counters)

```prometheus
# HELP technitium_queries_total Total DNS queries processed
# TYPE technitium_queries_total counter
technitium_queries_total 123456

# HELP technitium_responses_total DNS responses by response code
# TYPE technitium_responses_total counter
technitium_responses_total{rcode="noerror"} 120000
technitium_responses_total{rcode="servfail"} 100
technitium_responses_total{rcode="nxdomain"} 3000
technitium_responses_total{rcode="refused"} 50

# HELP technitium_queries_by_type_total DNS queries by resolution type
# TYPE technitium_queries_by_type_total counter
technitium_queries_by_type_total{type="authoritative"} 5000
technitium_queries_by_type_total{type="recursive"} 118000
technitium_queries_by_type_total{type="cached"} 80000
technitium_queries_by_type_total{type="blocked"} 10000
technitium_queries_by_type_total{type="dropped"} 5
```

### Blocking Metrics

```prometheus
# HELP technitium_blocked_queries_total Total blocked DNS queries
# TYPE technitium_blocked_queries_total counter
technitium_blocked_queries_total 10000

# HELP technitium_blocklist_domains Number of domains in blocklists
# TYPE technitium_blocklist_domains gauge
technitium_blocklist_domains 150000

# HELP technitium_blocked_zones Number of blocked zones configured
# TYPE technitium_blocked_zones gauge
technitium_blocked_zones 3

# HELP technitium_allowed_zones Number of allowed zones configured
# TYPE technitium_allowed_zones gauge
technitium_allowed_zones 0
```

### Cache Metrics (Gauges)

```prometheus
# HELP technitium_cache_entries Current number of entries in cache
# TYPE technitium_cache_entries gauge
technitium_cache_entries 5000
```

### Client Metrics (Gauges)

```prometheus
# HELP technitium_clients_total Total unique clients seen
# TYPE technitium_clients_total gauge
technitium_clients_total 25
```

### Zone Metrics (Gauges)

```prometheus
# HELP technitium_zones Total number of zones
# TYPE technitium_zones gauge
technitium_zones 5
```

### Exporter Metadata

```prometheus
# HELP technitium_up Whether the Technitium server is reachable
# TYPE technitium_up gauge
technitium_up 1

# HELP technitium_scrape_duration_seconds Time taken to scrape metrics
# TYPE technitium_scrape_duration_seconds gauge
technitium_scrape_duration_seconds 0.023
```

---

## Architecture

```text
technitium_exporter/
├── cmd/
│   └── technitium_exporter/
│       └── main.go              # Entry point, flag parsing, HTTP server
├── collector/
│   └── collector.go             # Prometheus collector implementation
├── config/
│   └── config.go                # Configuration struct and loading
├── exporter/
│   └── exporter.go              # HTTP handlers, landing page
├── internal/
│   └── technitium/
│       ├── client.go            # Technitium API client
│       └── types.go             # API response structs
└── test/
    └── integration_test.go      # Integration tests with mock server
```

---

## Implementation Plan

### Phase 1: Core Infrastructure

#### 1.1 Configuration (`config/config.go`)

```go
package config

import (
    "time"
    "github.com/alecthomas/kingpin/v2"
)

type Config struct {
    TechnitiumURL   string
    TechnitiumToken string
    ListenAddress   string
    MetricsPath     string
    ScrapeTimeout   time.Duration
}

func NewConfig() *Config {
    // Define kingpin flags with env var fallbacks
    // TECHNITIUM_URL, TECHNITIUM_TOKEN required
    // Defaults: :9167, /metrics, 10s timeout
}
```

**Flags:**

- `--technitium.url` / `TECHNITIUM_URL` (required)
- `--technitium.token` / `TECHNITIUM_TOKEN` (required)
- `--web.listen-address` / `LISTEN_ADDRESS` (default: `:9167`)
- `--web.telemetry-path` / `METRICS_PATH` (default: `/metrics`)
- `--scrape.timeout` / `SCRAPE_TIMEOUT` (default: `10s`)

#### 1.2 Technitium API Client (`pkg/technitium/client.go`)

```go
package technitium

import (
    "context"
    "net/http"
)

type Client struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

func NewClient(baseURL, token string, timeout time.Duration) *Client

// GetStats fetches dashboard statistics
func (c *Client) GetStats(ctx context.Context) (*StatsResponse, error)

// GetSettings fetches server settings (version, uptime)
func (c *Client) GetSettings(ctx context.Context) (*SettingsResponse, error)
```

**Implementation Notes:**

- Use `context.Context` for timeout/cancellation
- Check HTTP status codes, return descriptive errors
- Parse JSON into typed structs
- Close response bodies (per Prometheus best practices)

#### 1.3 API Response Types (`pkg/technitium/types.go`)

```go
package technitium

type StatsResponse struct {
    Response struct {
        Stats struct {
            TotalQueries       int64 `json:"totalQueries"`
            TotalNoError       int64 `json:"totalNoError"`
            TotalServerFailure int64 `json:"totalServerFailure"`
            TotalNxDomain      int64 `json:"totalNxDomain"`
            TotalRefused       int64 `json:"totalRefused"`
            TotalAuthoritative int64 `json:"totalAuthoritative"`
            TotalRecursive     int64 `json:"totalRecursive"`
            TotalCached        int64 `json:"totalCached"`
            TotalBlocked       int64 `json:"totalBlocked"`
            TotalDropped       int64 `json:"totalDropped"`
            TotalClients       int64 `json:"totalClients"`
            Zones              int64 `json:"zones"`
            CachedEntries      int64 `json:"cachedEntries"`
            AllowedZones       int64 `json:"allowedZones"`
            BlockedZones       int64 `json:"blockedZones"`
            BlockListZones     int64 `json:"blockListZones"`
        } `json:"stats"`
    } `json:"response"`
    Status string `json:"status"`
}

type SettingsResponse struct {
    Response struct {
        Version         string `json:"version"`
        Uptimestamp     string `json:"uptimestamp"`
        DnsServerDomain string `json:"dnsServerDomain"`
    } `json:"response"`
    Status string `json:"status"`
}
```

### Phase 2: Prometheus Collector

#### 2.1 Collector Implementation (`collector/collector.go`)

```go
package collector

import (
    "context"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/donaldgifford/technitium_exporter/pkg/technitium"
)

const namespace = "technitium"

type Collector struct {
    client *technitium.Client

    // Metric descriptors (defined once, reused)
    up                 *prometheus.Desc
    scrapeDuration     *prometheus.Desc
    serverInfo         *prometheus.Desc
    queriesTotal       *prometheus.Desc
    responsesTotal     *prometheus.Desc
    queriesByType      *prometheus.Desc
    blockedTotal       *prometheus.Desc
    blocklistDomains   *prometheus.Desc
    blockedZones       *prometheus.Desc
    allowedZones       *prometheus.Desc
    cacheEntries       *prometheus.Desc
    clients            *prometheus.Desc
    zones              *prometheus.Desc
}

func NewCollector(client *technitium.Client) *Collector {
    return &Collector{
        client: client,
        up: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "up"),
            "Whether the Technitium server is reachable",
            nil, nil,
        ),
        scrapeDuration: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
            "Time taken to scrape metrics from Technitium",
            nil, nil,
        ),
        serverInfo: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "server", "info"),
            "Technitium DNS server information",
            []string{"version", "server_domain"}, nil,
        ),
        queriesTotal: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "queries", "total"),
            "Total DNS queries processed",
            nil, nil,
        ),
        responsesTotal: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "responses", "total"),
            "DNS responses by response code",
            []string{"rcode"}, nil,
        ),
        queriesByType: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "queries_by_type", "total"),
            "DNS queries by resolution type",
            []string{"type"}, nil,
        ),
        blockedTotal: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "blocked_queries", "total"),
            "Total blocked DNS queries",
            nil, nil,
        ),
        blocklistDomains: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "blocklist_domains"),
            "Number of domains in blocklists",
            nil, nil,
        ),
        blockedZones: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "blocked_zones"),
            "Number of blocked zones configured",
            nil, nil,
        ),
        allowedZones: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "allowed_zones"),
            "Number of allowed zones configured",
            nil, nil,
        ),
        cacheEntries: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "cache", "entries"),
            "Current number of entries in cache",
            nil, nil,
        ),
        clients: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "clients", "total"),
            "Total unique clients seen",
            nil, nil,
        ),
        zones: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "zones"),
            "Total number of zones",
            nil, nil,
        ),
    }
}

// Describe sends all metric descriptors to the channel
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.up
    ch <- c.scrapeDuration
    ch <- c.serverInfo
    ch <- c.queriesTotal
    ch <- c.responsesTotal
    ch <- c.queriesByType
    ch <- c.blockedTotal
    ch <- c.blocklistDomains
    ch <- c.blockedZones
    ch <- c.allowedZones
    ch <- c.cacheEntries
    ch <- c.clients
    ch <- c.zones
}

// Collect fetches metrics from Technitium and sends them to channel
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
    start := time.Now()
    ctx := context.Background()

    // Fetch stats and settings concurrently
    var wg sync.WaitGroup
    var stats *technitium.StatsResponse
    var settings *technitium.SettingsResponse
    var statsErr, settingsErr error

    wg.Add(2)
    go func() {
        defer wg.Done()
        stats, statsErr = c.client.GetStats(ctx)
    }()
    go func() {
        defer wg.Done()
        settings, settingsErr = c.client.GetSettings(ctx)
    }()
    wg.Wait()

    duration := time.Since(start).Seconds()
    ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, duration)

    // If either request failed, mark as down
    if statsErr != nil || settingsErr != nil {
        ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
        return
    }

    ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)

    // Server info
    ch <- prometheus.MustNewConstMetric(
        c.serverInfo, prometheus.GaugeValue, 1,
        settings.Response.Version,
        settings.Response.DnsServerDomain,
    )

    s := stats.Response.Stats

    // Query totals
    ch <- prometheus.MustNewConstMetric(c.queriesTotal, prometheus.CounterValue, float64(s.TotalQueries))

    // Response codes
    ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalNoError), "noerror")
    ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalServerFailure), "servfail")
    ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalNxDomain), "nxdomain")
    ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalRefused), "refused")

    // Query types
    ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalAuthoritative), "authoritative")
    ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalRecursive), "recursive")
    ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalCached), "cached")
    ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalBlocked), "blocked")
    ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalDropped), "dropped")

    // Blocking stats
    ch <- prometheus.MustNewConstMetric(c.blockedTotal, prometheus.CounterValue, float64(s.TotalBlocked))
    ch <- prometheus.MustNewConstMetric(c.blocklistDomains, prometheus.GaugeValue, float64(s.BlockListZones))
    ch <- prometheus.MustNewConstMetric(c.blockedZones, prometheus.GaugeValue, float64(s.BlockedZones))
    ch <- prometheus.MustNewConstMetric(c.allowedZones, prometheus.GaugeValue, float64(s.AllowedZones))

    // Cache, clients, zones
    ch <- prometheus.MustNewConstMetric(c.cacheEntries, prometheus.GaugeValue, float64(s.CachedEntries))
    ch <- prometheus.MustNewConstMetric(c.clients, prometheus.GaugeValue, float64(s.TotalClients))
    ch <- prometheus.MustNewConstMetric(c.zones, prometheus.GaugeValue, float64(s.Zones))
}
```

### Phase 3: HTTP Server & Entry Point

#### 3.1 HTTP Handlers (`exporter/exporter.go`)

```go
package exporter

import (
    "net/http"
)

// LandingPage returns a simple HTML page with link to metrics
func LandingPage(metricsPath string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Technitium Exporter</title></head>
<body>
<h1>Technitium DNS Exporter</h1>
<p><a href="` + metricsPath + `">Metrics</a></p>
</body>
</html>`))
    }
}
```

#### 3.2 Main Entry Point (`cmd/technitium_exporter/main.go`)

```go
package main

import (
    "fmt"
    "net/http"
    "os"

    "github.com/go-kit/log"
    "github.com/go-kit/log/level"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/prometheus/common/promlog"
    "github.com/prometheus/common/promlog/flag"
    "github.com/prometheus/common/version"
    "github.com/prometheus/exporter-toolkit/web"
    "github.com/alecthomas/kingpin/v2"

    "github.com/donaldgifford/technitium_exporter/collector"
    "github.com/donaldgifford/technitium_exporter/config"
    "github.com/donaldgifford/technitium_exporter/exporter"
    "github.com/donaldgifford/technitium_exporter/pkg/technitium"
)

func main() {
    cfg := config.NewConfig()

    promlogConfig := &promlog.Config{}
    flag.AddFlags(kingpin.CommandLine, promlogConfig)
    kingpin.Version(version.Print("technitium_exporter"))
    kingpin.HelpFlag.Short('h')
    kingpin.Parse()

    logger := promlog.New(promlogConfig)

    level.Info(logger).Log("msg", "Starting technitium_exporter", "version", version.Info())

    // Validate required config
    if cfg.TechnitiumURL == "" || cfg.TechnitiumToken == "" {
        level.Error(logger).Log("msg", "TECHNITIUM_URL and TECHNITIUM_TOKEN are required")
        os.Exit(1)
    }

    // Create Technitium client
    client := technitium.NewClient(cfg.TechnitiumURL, cfg.TechnitiumToken, cfg.ScrapeTimeout)

    // Create and register collector
    coll := collector.NewCollector(client)
    prometheus.MustRegister(coll)
    prometheus.MustRegister(version.NewCollector("technitium_exporter"))

    // HTTP handlers
    http.Handle(cfg.MetricsPath, promhttp.Handler())
    http.HandleFunc("/", exporter.LandingPage(cfg.MetricsPath))

    level.Info(logger).Log("msg", "Listening on", "address", cfg.ListenAddress)
    if err := http.ListenAndServe(cfg.ListenAddress, nil); err != nil {
        level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
        os.Exit(1)
    }
}
```

### Phase 4: Testing

#### 4.1 Unit Tests

**`pkg/technitium/client_test.go`:**

- Test successful API responses
- Test HTTP error handling
- Test JSON parsing errors
- Test timeout behavior

**`collector/collector_test.go`:**

- Test metric collection with mock client
- Test error handling (partial failures)
- Verify metric names and labels

#### 4.2 Integration Tests

**`test/integration_test.go`:**

- Mock HTTP server returning realistic responses
- End-to-end scrape validation
- Verify Prometheus metric format

---

## Dependencies

```go
// go.mod additions
require (
    github.com/alecthomas/kingpin/v2 v2.4.0
    github.com/go-kit/log v0.2.1
    github.com/prometheus/client_golang v1.19.0
    github.com/prometheus/common v0.51.1
    github.com/prometheus/exporter-toolkit v0.11.0
)
```

---

## Build & Test Commands

```bash
# Build
make build
# or: go build -o technitium_exporter ./cmd/technitium_exporter

# Run tests
go test -v -race ./...

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint
golangci-lint run

# Local run
export TECHNITIUM_URL=http://localhost:5380
export TECHNITIUM_TOKEN=your-token
./technitium_exporter
```

---

## v0.0.1 Acceptance Criteria

1. **Exporter starts** with valid config (URL + token)
2. **Exporter fails fast** with clear error if config missing
3. **`/metrics` endpoint** returns valid Prometheus format
4. **`/` endpoint** returns HTML landing page
5. **All MVP metrics** are exposed when Technitium is reachable
6. **`technitium_up{} 0`** when Technitium is unreachable
7. **Scrape completes** within default 10s timeout
8. **No race conditions** (verified with `-race` flag)
9. **golangci-lint passes** with no errors
10. **Unit tests pass** with >70% coverage on collector package

---

## Out of Scope for v0.0.1

- Per-zone metrics
- Top domains/clients metrics
- DHCP metrics
- Query log shipping
- Multi-instance support
- TLS configuration
- Helm chart
- Grafana dashboard (separate deliverable)

---

## Implementation Order

1. `config/config.go` - Configuration loading
2. `pkg/technitium/types.go` - API response structs
3. `pkg/technitium/client.go` - HTTP client
4. `pkg/technitium/client_test.go` - Client tests
5. `collector/collector.go` - Prometheus collector
6. `collector/collector_test.go` - Collector tests
7. `exporter/exporter.go` - HTTP handlers
8. `cmd/technitium_exporter/main.go` - Entry point
9. Integration tests
10. Final lint/test pass

---

## Implementation Notes

1. **Stats type parameter**: We'll experiment with the `type` parameter during
   implementation to find what provides lifetime/cumulative totals for counter
   metrics. If only windowed stats are available, we'll adjust metric types
   accordingly (gauges instead of counters). The API behavior will guide our
   final metric type decisions.

2. **Scrape interval**: Target 15s scrape intervals. We'll validate this works
   without issues during testing.

3. **Token handling**: For v0.0.1, use environment variables and CLI flags only.
   Token-file support (for Kubernetes secrets) can be added in a future version
   if needed.

---

## References

- [Prometheus Exporter Best Practices](https://prometheus.io/docs/instrumenting/writing_exporters/)
- [Technitium API Documentation](https://github.com/TechnitiumSoftware/DnsServer/blob/master/APIDOCS.md)
- [prometheus/client_golang](https://github.com/prometheus/client_golang)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
