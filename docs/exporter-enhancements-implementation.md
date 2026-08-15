# Exporter Enhancements: Implementation Plan

Reference: [docs/exporter-enhancements-plan.md](exporter-enhancements-plan.md)

## Overview

This plan implements 6 new metric families from the existing
`/api/dashboard/stats/get` response, adds a `--collector.top-entries` flag, and
exposes server uptime from the settings endpoint. No new API calls are needed.

**New metrics:**

| Metric                                    | Type    | Source                  |
| ----------------------------------------- | ------- | ----------------------- |
| `technitium_queries_by_record_type_total` | Counter | `queryTypeChartData`    |
| `technitium_queries_by_protocol_total`    | Counter | `protocolTypeChartData` |
| `technitium_top_clients_hits`             | Gauge   | `topClients`            |
| `technitium_top_domains_hits`             | Gauge   | `topDomains`            |
| `technitium_top_blocked_domains_hits`     | Gauge   | `topBlockedDomains`     |
| `technitium_server_uptime_seconds`        | Gauge   | `settings.uptimestamp`  |

---

## Phase 1: Types

**Goal:** Expand `StatsResponseData` to capture the unparsed API fields and add
new types for top-N entries.

**Files:** `pkg/technitium/types.go`

**Changes:**

1. Add fields to `StatsResponseData` (line 12):
   - `QueryTypeChartData map[string]int64 json:"queryTypeChartData"`
   - `ProtocolTypeChartData map[string]int64 json:"protocolTypeChartData"`
   - `TopClients []TopClient json:"topClients"`
   - `TopDomains []TopEntry json:"topDomains"`
   - `TopBlockedDomains []TopEntry json:"topBlockedDomains"`

2. Add `TopClient` struct:
   - `Name string json:"name"`
   - `Hits int64 json:"hits"`
   - `RateLimited bool json:"rateLimited"`

3. Add `TopEntry` struct:
   - `Name string json:"name"`
   - `Hits int64 json:"hits"`

**Design notes:**

- `map[string]int64` for chart data because keys are dynamic (API may return new
  record types or protocols without code changes)
- `TopClient` is separate from `TopEntry` because of the `rateLimited` field
- `TopEntry` is reused for both `topDomains` and `topBlockedDomains`

**Success criteria:**

- [x] `go build ./...` compiles
- [x] `go vet ./...` passes
- [x] Existing tests still pass: `go test ./pkg/technitium/...`

---

## Phase 2: Client-level parsing tests

**Goal:** Verify that `json.Unmarshal` correctly populates the new
`StatsResponseData` fields from a realistic API response.

**Files:** `pkg/technitium/client_test.go`

**Changes:**

1. Add `TestStatsResponse_ChartData` test:
   - JSON fixture includes `queryTypeChartData`, `protocolTypeChartData`,
     `topClients`, `topDomains`, `topBlockedDomains` alongside existing `stats`
   - Assert `QueryTypeChartData["A"]` == expected value
   - Assert `ProtocolTypeChartData["UDP"]` == expected value
   - Assert `len(TopClients)` and first entry's `Name`, `Hits`, `RateLimited`
   - Assert `len(TopDomains)` and first entry's `Name`, `Hits`
   - Assert `len(TopBlockedDomains)` and first entry's `Name`, `Hits`

2. Add `TestStatsResponse_EmptyChartData` test:
   - JSON fixture with `stats` only (no chart data fields) -- verifies backward
     compatibility (maps should be nil, slices should be nil)

**Success criteria:**

- [x] `go test -v -run TestStatsResponse_ChartData ./pkg/technitium/...` passes
- [x] `go test -v -run TestStatsResponse_EmptyChartData ./pkg/technitium/...`
      passes
- [x] All existing client tests still pass

---

## Phase 3: Collector -- descriptors and constructor

**Goal:** Add 6 new metric descriptors and the `topEntriesEnabled` flag to the
collector. Update `NewCollector` signature to accept `topEntries bool`.

**Files:** `collector/collector.go`, `cmd/technitium_exporter/main.go`

**Changes to `collector/collector.go`:**

1. Add imports: `"strconv"`, `"strings"`, `"time"` (as needed)

2. Add 6 new fields to `Collector` struct (after line 35):
   - `queriesByRecordType *prometheus.Desc`
   - `queriesByProtocol *prometheus.Desc`
   - `topClients *prometheus.Desc`
   - `topDomains *prometheus.Desc`
   - `topBlockedDomains *prometheus.Desc`
   - `uptimeSeconds *prometheus.Desc`
   - `topEntriesEnabled bool`

3. Update `NewCollector` signature (line 39):
   - From:
     `func NewCollector(client *technitium.Client, logger *slog.Logger) *Collector`
   - To:
     `func NewCollector(client *technitium.Client, logger *slog.Logger, topEntries bool) *Collector`

4. Add descriptor initializations in `NewCollector`:
   - `queriesByRecordType`: always initialized
     - FQName: `technitium_queries_by_record_type_total`
     - Labels: `["record_type"]`
   - `queriesByProtocol`: always initialized
     - FQName: `technitium_queries_by_protocol_total`
     - Labels: `["protocol"]`
   - `uptimeSeconds`: always initialized
     - FQName: `technitium_server_uptime_seconds`
     - Labels: none
   - `topClients`: only if `topEntries == true`
     - FQName: `technitium_top_clients_hits`
     - Labels: `["client", "rate_limited"]`
   - `topDomains`: only if `topEntries == true`
     - FQName: `technitium_top_domains_hits`
     - Labels: `["domain"]`
   - `topBlockedDomains`: only if `topEntries == true`
     - FQName: `technitium_top_blocked_domains_hits`
     - Labels: `["domain"]`

5. Update `Describe()` (line 112):
   - Add `ch <- c.queriesByRecordType`
   - Add `ch <- c.queriesByProtocol`
   - Add `ch <- c.uptimeSeconds`
   - Guarded:
     `if c.topEntriesEnabled { ch <- c.topClients; ch <- c.topDomains; ch <- c.topBlockedDomains }`

**Changes to `cmd/technitium_exporter/main.go`:**

1. Add `--collector.top-entries` flag (after line 40, in the flag definition
   area):

   ```go
   topEntries := app.Flag("collector.top-entries",
       "Enable top clients/domains/blocked domains metrics.").
       Default("true").Bool()
   ```

2. Update collector construction (line 86):
   - From: `collector.NewCollector(client, logger)`
   - To: `collector.NewCollector(client, logger, *topEntries)`

**Success criteria:**

- [x] `go build ./...` compiles
- [x] `go vet ./...` passes
- [x] Existing tests compile (will need NewCollector call sites updated -- see
      note below)

**Note:** All existing test call sites of `NewCollector(client, logger)` will
break because the signature changes. These must be updated to
`NewCollector(client, logger, true)` in this phase to keep tests compiling. This
is a mechanical find-and-replace in `collector/collector_test.go`.

---

## Phase 4: Collector -- collection logic

**Goal:** Add the metric emission logic in `Collect()` for all 6 new metrics.

**Files:** `collector/collector.go`

**Changes to `Collect()` (after line 206):**

1. **Query type chart data** -- iterate `stats.Response.QueryTypeChartData`:

   ```go
   for recordType, count := range stats.Response.QueryTypeChartData {
       ch <- prometheus.MustNewConstMetric(
           c.queriesByRecordType, prometheus.CounterValue,
           float64(count), recordType,
       )
   }
   ```

2. **Protocol type chart data** -- iterate
   `stats.Response.ProtocolTypeChartData`:

   ```go
   for protocol, count := range stats.Response.ProtocolTypeChartData {
       ch <- prometheus.MustNewConstMetric(
           c.queriesByProtocol, prometheus.CounterValue,
           float64(count), strings.ToLower(protocol),
       )
   }
   ```

3. **Top entries** -- guarded by `c.topEntriesEnabled`:

   ```go
   if c.topEntriesEnabled {
       for _, client := range stats.Response.TopClients {
           ch <- prometheus.MustNewConstMetric(
               c.topClients, prometheus.GaugeValue,
               float64(client.Hits), client.Name,
               strconv.FormatBool(client.RateLimited),
           )
       }
       for _, domain := range stats.Response.TopDomains {
           ch <- prometheus.MustNewConstMetric(
               c.topDomains, prometheus.GaugeValue,
               float64(domain.Hits), domain.Name,
           )
       }
       for _, domain := range stats.Response.TopBlockedDomains {
           ch <- prometheus.MustNewConstMetric(
               c.topBlockedDomains, prometheus.GaugeValue,
               float64(domain.Hits), domain.Name,
           )
       }
   }
   ```

4. **Uptime** -- after the existing settings handling block (around line 177):

   ```go
   if settingsErr == nil {
       startTime, err := time.Parse(time.RFC3339, settings.Response.Uptimestamp)
       if err == nil {
           uptime := time.Since(startTime).Seconds()
           ch <- prometheus.MustNewConstMetric(
               c.uptimeSeconds, prometheus.GaugeValue, uptime,
           )
       }
   }
   ```

**Success criteria:**

- [x] `go build ./...` compiles
- [x] `go vet ./...` passes
- [x] `make lint` passes (no new golangci-lint issues)
- [x] Existing tests still pass (no new data in fixtures yet, so new loops are
      no-ops on nil maps/slices)

---

## Phase 5: Collector tests

**Goal:** Update all test fixtures with chart data fields and add new test cases
verifying the 6 new metrics.

**Files:** `collector/collector_test.go`

**Changes:**

1. **Update `realWorldStatsJSON()`** -- add chart data fields to the `response`
   object (after the `stats` block):

   ```json
   "queryTypeChartData": {"A": 40, "AAAA": 25, "PTR": 5, "TXT": 2},
   "protocolTypeChartData": {"UDP": 65, "TCP": 7},
   "topClients": [
     {"name": "10.10.11.18", "hits": 50, "rateLimited": false},
     {"name": "10.10.11.1", "hits": 22, "rateLimited": false}
   ],
   "topDomains": [
     {"name": "dns03.fartlab.dev", "hits": 30},
     {"name": "example.com", "hits": 15}
   ],
   "topBlockedDomains": [
     {"name": "ads.example.com", "hits": 8}
   ]
   ```

2. **Update `highTrafficStatsJSON()`** -- add chart data fields with larger
   numbers:

   ```json
   "queryTypeChartData": {"A": 800000, "AAAA": 500000, "TXT": 100000, "HTTPS": 50000, "PTR": 30000, "SRV": 2000},
   "protocolTypeChartData": {"UDP": 1400000, "TCP": 123456},
   "topClients": [
     {"name": "10.0.0.1", "hits": 250000, "rateLimited": false},
     {"name": "10.0.0.2", "hits": 180000, "rateLimited": true}
   ],
   "topDomains": [
     {"name": "api.github.com", "hits": 50000},
     {"name": "dns.google", "hits": 30000}
   ],
   "topBlockedDomains": [
     {"name": "stats.grafana.org", "hits": 25000},
     {"name": "telemetry.example.com", "hits": 15000}
   ]
   ```

3. **Update `NewCollector` call sites** (already done in Phase 3 note, but
   verify): all `NewCollector(client, newTestLogger())` become
   `NewCollector(client, newTestLogger(), true)`.

4. **Update metric count in `TestCollector_Collect_RealWorldData`** (line 136):
   - Old: 20 metrics
   - New: 20 (existing) + 4 (queryTypeChartData: A, AAAA, PTR, TXT) + 2
     (protocolTypeChartData: UDP, TCP) + 2 (topClients) + 2 (topDomains) + 1
     (topBlockedDomains) + 1 (uptimeSeconds) = 32 metrics

5. **Update descriptor count in `TestCollector_Describe`** (line 273):
   - Old: 13 descriptors
   - New: 13 + 6 (queriesByRecordType, queriesByProtocol, topClients,
     topDomains, topBlockedDomains, uptimeSeconds) = 19 descriptors

6. **Add `TestCollector_QueriesByRecordType`** -- verify record type metrics
   using `testutil.CollectAndCompare` against `highTrafficStatsJSON()`:

   ```text
   technitium_queries_by_record_type_total{record_type="A"} 800000
   technitium_queries_by_record_type_total{record_type="AAAA"} 500000
   ...
   ```

7. **Add `TestCollector_QueriesByProtocol`** -- verify protocol metrics:

   ```text
   technitium_queries_by_protocol_total{protocol="udp"} 1400000
   technitium_queries_by_protocol_total{protocol="tcp"} 123456
   ```

   (Note: label values are lowercased)

8. **Add `TestCollector_TopClients`** -- verify top clients metrics:

   ```text
   technitium_top_clients_hits{client="10.0.0.1",rate_limited="false"} 250000
   technitium_top_clients_hits{client="10.0.0.2",rate_limited="true"} 180000
   ```

9. **Add `TestCollector_TopDomains`** -- verify top domains metrics.

10. **Add `TestCollector_TopBlockedDomains`** -- verify top blocked domains
    metrics.

11. **Add `TestCollector_TopEntriesDisabled`** -- create collector with
    `NewCollector(client, logger, false)`:
    - Verify descriptor count is 16 (19 - 3 top-entry descriptors)
    - Verify top_clients, top_domains, top_blocked_domains metrics are NOT
      emitted
    - Verify record_type, protocol, and uptime metrics ARE still emitted

12. **Add `TestCollector_UptimeSeconds`** -- verify uptime metric is emitted
    when settings succeed:
    - Use `realWorldSettingsJSON()` which has
      `"uptimestamp": "2024-01-15T10:30:00Z"`
    - Verify `technitium_server_uptime_seconds` is a positive value (exact value
      depends on wall clock, so just check it exists and is > 0)

13. **Update `TestStatsResponse_Unmarshal`** (line 422):
    - Update `realWorldStatsJSON()` already has new fields from step 1
    - Add assertions for `resp.Response.QueryTypeChartData["A"]`,
      `len(resp.Response.TopClients)`, etc.

**Success criteria:**

- [x] `go test -v -race ./collector/...` -- all tests pass
- [x] `go test -v -race ./pkg/technitium/...` -- all tests pass
- [x] `make test` -- full test suite green
- [x] `make lint` -- no lint issues

---

## Phase 6: Full verification

**Goal:** Run the complete CI-equivalent check locally.

**Commands:**

```bash
make fmt
make lint
make test-coverage
make build
make security
```

**Success criteria:**

- [x] `make fmt` -- no formatting changes
- [x] `make lint` -- clean (0 issues)
- [x] `make test-coverage` -- all tests pass (collector: 97.8%, client: 91.5%)
- [x] `make build` -- binary builds successfully
- [x] `make security` -- no vulnerabilities (govulncheck clean, trivy 0
      findings)
- [x] Manual smoke test: run the built binary against a test server (if
      available) and curl `/metrics` to verify new metrics appear

---

## Phase 7: Documentation and dashboard (deferred)

**Goal:** Update CLAUDE.md metrics table and Grafana dashboard with new panels.

**Note:** This phase is deferred to a follow-up PR to keep the code changes
focused and reviewable. The dashboard JSON changes are large and independent of
the collector logic.

**Files (when ready):**

- `CLAUDE.md` -- add 6 new metrics to the table
- `contrib/grafana/technitium-dashboard.json` -- add panels:
  - Query Types piechart (donut)
  - Protocol Distribution stat panel
  - Top Clients table
  - Top Domains table
  - Top Blocked Domains table
  - Server Uptime stat panel

**Success criteria:**

- [ ] CLAUDE.md metrics table matches actual output
- [ ] Dashboard imports without errors in Grafana
- [ ] All new panels render with data

---

## Dependency Graph

```text
Phase 1 (types)
    |
    v
Phase 2 (client parsing tests)
    |
    v
Phase 3 (collector descriptors + constructor + main.go flag)
    |
    v
Phase 4 (collector collection logic)
    |
    v
Phase 5 (collector tests)
    |
    v
Phase 6 (full verification)
    |
    v
Phase 7 (docs + dashboard -- separate PR)
```

Each phase builds on the previous. Phases 1-6 are one PR. Phase 7 is a
follow-up.

## Open Questions

None -- all decisions were resolved in the
[enhancements plan](exporter-enhancements-plan.md#decisions).
