# Loki + Alloy Integration Plan: DNS Query Log Analytics

## Context

The Unbound dashboard has 8 panels powered by Loki log queries that provide
real-time DNS query visibility. These panels cannot be replicated with
Prometheus metrics alone -- they require per-query log data.

This plan covers:

1. How Technitium exposes query logs
2. Alloy pipeline to ship logs to Loki
3. Loki label and field strategy
4. Dashboard panels to add
5. What we ship vs. what users configure

## Prerequisites

This plan depends on the exporter enhancements plan
(`docs/exporter-enhancements-plan.md`) being completed first. Several panels
that the Unbound dashboard implements via Loki (Top Clients, Top Blocked
Domains) are already available as Prometheus metrics from the Technitium API.
The Loki integration adds **live log tailing and per-query detail** that metrics
cannot provide.

## Technitium Query Log Sources

Technitium's **Log Exporter App** (installed via Apps section in admin UI)
provides three log sinks. All output the same structured JSON format.

### Log Exporter App Configuration

```json
{
  "maxQueueSize": 1000000,
  "ebableEdnsLogging": false,
  "file": {
    "path": "./dns_logs.json",
    "enabled": false
  },
  "http": {
    "endpoint": "http://localhost:5000/logs",
    "headers": {
      "Authorization": "Bearer abc123"
    },
    "enabled": false
  },
  "syslog": {
    "address": "127.0.0.1",
    "port": 514,
    "protocol": "UDP",
    "enabled": false
  }
}
```

**Note:** `ebableEdnsLogging` is a typo in Technitium's config (should be
"enable"). When set to `true`, adds EDNS/EDE fields with block reasons to log
entries.

### Log Format (Verified from Live Instance)

Each log entry is a single-line JSON object:

```json
{
  "answers": [
    {
      "dnssecStatus": "Insecure",
      "name": "play.google.com",
      "recordClass": "IN",
      "recordData": "142.250.177.78",
      "recordTtl": 50,
      "recordType": "A"
    }
  ],
  "clientIp": "10.10.10.91",
  "edns": [],
  "protocol": "Tcp",
  "question": {
    "questionClass": "IN",
    "questionName": "play.google.com",
    "questionType": "A"
  },
  "responseCode": "NoError",
  "responseRtt": 13.498,
  "responseType": "Recursive",
  "timestamp": "2026-02-09T15:17:14.193Z"
}
```

**Fields present in every entry:**

| Field                    | Type   | Values                                             |
| ------------------------ | ------ | -------------------------------------------------- |
| `clientIp`               | string | Client IP address                                  |
| `protocol`               | string | `Udp`, `Tcp` (title-case)                          |
| `responseCode`           | string | `NoError`, `NxDomain`, `ServerFailure`, `Refused`  |
| `responseType`           | string | `Authoritative`, `Cached`, `Recursive`, `Blocked`  |
| `question.questionName`  | string | Queried domain                                     |
| `question.questionType`  | string | `A`, `AAAA`, `TXT`, `SOA`, `HTTPS`, etc.           |
| `question.questionClass` | string | `IN`                                               |
| `timestamp`              | string | ISO 8601                                           |
| `answers`                | array  | Answer records (empty on NxDomain/Blocked)         |
| `edns`                   | array  | EDNS data (empty unless `ebableEdnsLogging: true`) |

**Conditional fields:**

| Field         | Type  | When Present                                                                                                   |
| ------------- | ----- | -------------------------------------------------------------------------------------------------------------- |
| `responseRtt` | float | **Recursive queries only** -- response time in milliseconds. Not present on Cached, Authoritative, or Blocked. |

**Key finding:** `responseType: "Blocked"` directly identifies blocked queries.
No need to infer from response code.

### Option A: Syslog Forwarding (Recommended)

Configure Log Exporter App to send syslog to Alloy:

```json
"syslog": {
    "address": "<alloy-host>",
    "port": 1514,
    "protocol": "UDP",
    "enabled": true
}
```

**Advantages:**

- Push-based, low latency
- Alloy has native `loki.source.syslog` component
- No API polling overhead
- Same JSON format wrapped in syslog envelope

### Option B: File Sink + Alloy Tail

Configure Log Exporter App to write JSON Lines to file:

```json
"file": {
    "path": "./dns_logs.json",
    "enabled": true
}
```

Alloy tails the file using `local.file_match` + `loki.source.file`.

**Advantages:**

- No network config needed between Technitium and Alloy
- Works when they run on the same host or share a volume
- Same JSON format, no syslog envelope to strip

### Option C: Query Log API

Technitium exposes `/api/queryLogs/list` for paginated query log access.

**Advantages:**

- No Log Exporter App needed
- No Technitium config changes

**Disadvantages:**

- Poll-based (latency proportional to poll interval)
- Requires admin API token
- Pagination complexity

### Recommendation

**Syslog (Option A)** for production deployments with Alloy on a separate host.
**File sink (Option B)** for single-host or container deployments where Alloy
can mount the log volume. **API (Option C)** as fallback when the Log Exporter
App can't be installed.

## Alloy Pipeline Architecture

### Syslog Pipeline

```
Technitium DNS Server (Log Exporter App)
    │
    │ syslog (UDP/TCP) -- JSON payload in syslog envelope
    ▼
┌─────────────────────────┐
│  Grafana Alloy           │
│                          │
│  loki.source.syslog      │  ← Receive syslog messages
│         │                │
│  loki.process            │  ← Parse JSON, extract labels
│         │                │
│  loki.write              │  ← Ship to Loki
└─────────────────────────┘
    │
    ▼
  Loki
```

### File Tail Pipeline

```
Technitium DNS Server (Log Exporter App)
    │
    │ writes dns_logs.json (JSON Lines)
    ▼
┌─────────────────────────┐
│  Grafana Alloy           │
│                          │
│  local.file_match        │  ← Match log file path
│  loki.source.file        │  ← Tail the JSON Lines file
│         │                │
│  loki.process            │  ← Parse JSON, extract labels
│         │                │
│  loki.write              │  ← Ship to Loki
└─────────────────────────┘
    │
    ▼
  Loki
```

### Alloy Syslog Config (`contrib/alloy/syslog.alloy`)

```alloy
// Receive syslog from Technitium Log Exporter App
loki.source.syslog "technitium" {
  listener {
    address  = "0.0.0.0:1514"
    protocol = "udp"
    labels   = {
      job    = "technitium-syslog",
      source = "syslog",
    }
  }
  forward_to = [loki.process.technitium.receiver]
}

// Parse JSON log entries from Technitium
loki.process "technitium" {
  // The log line is a JSON object -- extract top-level fields
  stage.json {
    expressions = {
      client_ip     = "clientIp",
      protocol      = "protocol",
      response_code = "responseCode",
      response_type = "responseType",
      response_rtt  = "responseRtt",
      domain        = "question.questionName",
      record_type   = "question.questionType",
    }
  }

  // Lowercase protocol for label consistency (Udp -> udp, Tcp -> tcp)
  stage.template {
    source   = "protocol"
    template = "{{ ToLower .Value }}"
  }

  // Classify: dns="block" when responseType is "Blocked", dns="reply" otherwise
  stage.template {
    source   = "dns"
    template = "{{ if eq .response_type \"Blocked\" }}block{{ else }}reply{{ end }}"
  }

  // Promote low-cardinality fields to labels (indexed)
  stage.labels {
    values = {
      dns           = "",
      protocol      = "",
      response_code = "",
      response_type = "",
    }
  }

  // High-cardinality fields go to structured metadata (queryable, not indexed)
  stage.structured_metadata {
    values = {
      client_ip    = "",
      domain       = "",
      record_type  = "",
      response_rtt = "",
    }
  }

  forward_to = [loki.write.default.receiver]
}

loki.write "default" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

### Alloy File Tail Config (`contrib/alloy/file-tail.alloy`)

```alloy
// Match Technitium Log Exporter file output
local.file_match "technitium" {
  path_targets = [{
    __path__ = "/path/to/technitium/dns_logs.json",
    job      = "technitium-syslog",
    source   = "file",
  }]
}

// Tail the JSON Lines log file
loki.source.file "technitium" {
  targets    = local.file_match.technitium.targets
  forward_to = [loki.process.technitium.receiver]
}

// Same processing pipeline as syslog (reuse identical loki.process block)
// ... (identical to loki.process "technitium" above)
```

**Note:** Both syslog and file-tail pipelines use the same `loki.process` stage
and produce identical labels/metadata. Dashboards work with either source. The
`job` label is the same (`technitium-syslog`) so panels don't need to
differentiate.

### Label Strategy

Labels (indexed, low cardinality):

| Label           | Values                                            | Purpose                         |
| --------------- | ------------------------------------------------- | ------------------------------- |
| `job`           | `technitium-syslog`                               | Standard job label              |
| `source`        | `syslog`, `file`                                  | Which sink produced the log     |
| `dns`           | `reply`, `block`                                  | Primary query classification    |
| `protocol`      | `udp`, `tcp`                                      | Transport protocol (lowercased) |
| `response_code` | `NoError`, `NxDomain`, `ServerFailure`, `Refused` | DNS response code               |
| `response_type` | `Authoritative`, `Cached`, `Recursive`, `Blocked` | Resolution type                 |

Structured metadata (queryable, not indexed):

| Field          | Source                  | Purpose                                                |
| -------------- | ----------------------- | ------------------------------------------------------ |
| `client_ip`    | `clientIp`              | Client IP address (high cardinality)                   |
| `domain`       | `question.questionName` | Queried domain (high cardinality)                      |
| `record_type`  | `question.questionType` | DNS record type (A, AAAA, TXT, etc.)                   |
| `response_rtt` | `responseRtt`           | Response time in ms (Recursive only, absent otherwise) |

**Cardinality note:** `client_ip`, `domain`, and `record_type` are NOT labels.
Domain and client have unbounded cardinality. Use `| json` or structured
metadata filters in LogQL to filter/aggregate at query time.

## Dashboard Panels

### Panels Using Loki Data

These panels map to the Unbound dashboard's Loki-based panels. All queries use
structured metadata fields (`client_ip`, `domain`, `record_type`,
`response_rtt`) which are queryable without being indexed labels.

#### 1. Live Clients Timeline (Timeseries, stacked bars)

```logql
sum by(client_ip) (
  count_over_time({job="technitium-syslog", dns="reply"} [$__interval])
)
```

**Panel config:** Stacked bars, fill opacity 64%, hue gradient.

#### 2. Queries Distribution (Piechart, donut)

```logql
sum by(dns) (count_over_time({job="technitium-syslog"} [$__range]))
```

Two slices: `reply` and `block`.

**Note:** Overlaps with Prometheus "Allowed vs Blocked" panel. Loki version
counts every query from logs; Prometheus version uses API aggregate counters.
Both are useful.

#### 3. Queries Over Time (Timeseries, stacked bars)

```logql
# Reply queries
count_over_time({job="technitium-syslog", dns="reply"} [$__interval])

# Blocked queries
count_over_time({job="technitium-syslog", dns="block"} [$__interval])
```

**Panel config:** Stacked bars, fill opacity 64%, legend at bottom.

#### 4. Client Requests Over Time (Timeseries, stacked bars)

```logql
sum by(client_ip) (
  count_over_time({job="technitium-syslog", dns="reply"} [$__interval])
)
```

**Panel config:** Stacked bars, fill opacity 52%, legend on right showing client
IPs.

#### 5. Top Client Queries (Table)

```logql
sum by(domain, client_ip) (
  count_over_time({job="technitium-syslog", dns="reply"} [$__range])
)
```

**Columns:** Domain, Client, Count. Sorted by Count descending.

**Note:** Prometheus `technitium_top_clients_hits` provides client + hits (no
domain dimension). This Loki version adds the domain dimension for deeper
analysis.

#### 6. Top Blocked Domains (Table)

```logql
sum by(domain) (
  count_over_time({job="technitium-syslog", dns="block"} [$__range])
)
```

**Columns:** Domain, Count. Sorted descending.

**Note:** Overlaps with Prometheus `technitium_top_blocked_domains_hits`. Loki
version provides full history over the dashboard range rather than the API's
current top-N snapshot.

#### 7. Live Queries (Table)

```logql
{job="technitium-syslog", dns="reply"} | json
```

Displays: timestamp, clientIp, question.questionName, question.questionType,
responseCode, responseRtt, protocol.

**Panel config:** Table visualization, sorted by time descending, color-text
cells.

#### 8. Live Blocked Queries (Table)

```logql
{job="technitium-syslog", dns="block"} | json
```

Displays: timestamp, clientIp, question.questionName, question.questionType,
responseCode.

**Panel config:** Table visualization, sorted by time descending.

#### 9. Response Time (Timeseries) -- NEW, not in Unbound dashboard

```logql
# Average response time (Recursive queries only)
avg_over_time({job="technitium-syslog", response_type="Recursive"} | json | unwrap responseRtt [$__interval])

# P50 / P95 / P99 response time
quantile_over_time(0.50, {job="technitium-syslog", response_type="Recursive"} | json | unwrap responseRtt [$__interval])
quantile_over_time(0.95, {job="technitium-syslog", response_type="Recursive"} | json | unwrap responseRtt [$__interval])
quantile_over_time(0.99, {job="technitium-syslog", response_type="Recursive"} | json | unwrap responseRtt [$__interval])
```

**Note:** `responseRtt` is only present on Recursive queries. Filtering by
`response_type="Recursive"` avoids errors from missing field on
Cached/Authoritative/Blocked entries.

**Panel config:** Line chart, milliseconds unit, legend showing avg/p50/p95/p99.

#### 10. Response Type Distribution (Piechart, donut) -- NEW

```logql
sum by(response_type) (count_over_time({job="technitium-syslog"} [$__range]))
```

Four slices: Authoritative, Cached, Recursive, Blocked.

**Panel config:** Donut piechart, table legend on right.

## What We Ship vs. What Users Configure

### We Ship (in this repo)

| Artifact               | Location                                         | Description                                                 |
| ---------------------- | ------------------------------------------------ | ----------------------------------------------------------- |
| Default dashboard      | `contrib/grafana/technitium-dark-dashboard.json` | Exporter metrics + Log Exporter Loki row + API logs row     |
| Logs dashboard         | `contrib/grafana/technitium-logs-dashboard.json` | In-depth log analytics with filtering, both log source rows |
| Syslog Alloy config    | `contrib/alloy/syslog.alloy`                     | Alloy pipeline for syslog ingestion                         |
| File tail Alloy config | `contrib/alloy/file-tail.alloy`                  | Alloy pipeline for JSON Lines file tail                     |
| API logs Alloy config  | `contrib/alloy/api-logs.alloy`                   | Alloy pipeline for API log polling                          |
| Documentation          | `contrib/alloy/README.md`                        | Setup guide for all log sources                             |

### Users Configure (external to this repo)

| Component                   | User Responsibility                                                     |
| --------------------------- | ----------------------------------------------------------------------- |
| Technitium Log Exporter App | Install via Apps section in Technitium admin UI                         |
| Log Exporter config         | Enable syslog or file sink, set Alloy address (for Log Exporter source) |
| Technitium API token        | Admin token with query log permissions (for API log source, optional)   |
| Grafana Alloy deployment    | Deploy and configure Alloy with the provided config(s)                  |
| Loki instance               | Running Loki (or Grafana Cloud)                                         |
| Grafana Loki datasource     | Add Loki datasource in Grafana and select it in dashboard variable      |

### Dashboard Datasource Strategy

Both dashboards support three deployment tiers:

**Tier 1: Prometheus only** (no Loki)

- All exporter-based panels work standalone
- Syslog and API log rows stay collapsed / show "no data"

**Tier 2: Prometheus + syslog Loki** (recommended)

- Exporter panels + syslog log row auto-expands
- API log row stays collapsed

**Tier 3: Prometheus + syslog + API logs** (full visibility)

- All panels populated, both log rows expanded

Implementation:

- Add `loki_datasource` template variable (type: datasource, query: loki)
- Loki panels use `$loki_datasource`
- Syslog panels filter on `{job="technitium-syslog"}`
- API log panels filter on `{job="technitium-api-logs"}`
- Collapsible rows use Grafana's row repeat/collapse with "no data" handling

## What's Possible Now (Updated After Log Samples)

The `responseRtt` field (milliseconds, present on Recursive queries) enables
response time panels that were previously uncertain:

| Unbound Panel                              | Technitium Equivalent                                   | Status                                                                                                                   |
| ------------------------------------------ | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Recursion Time (avg/median timeseries)     | `avg_over_time` / `quantile_over_time` on `responseRtt` | **Possible** -- see Panel 9 above                                                                                        |
| Response Time Buckets (histogram bargauge) | LogQL `unwrap responseRtt` with bucket ranges           | **Partially possible** -- per-query values, not pre-bucketed like Unbound, but achievable with LogQL histogram functions |

## What's NOT Possible (Unbound-Specific)

| Unbound Panel                         | Why Not Available                                                      |
| ------------------------------------- | ---------------------------------------------------------------------- |
| Memory Usage (caches + modules bytes) | Not exposed by Technitium API or logs                                  |
| Cache Hit/Miss separate counters      | Technitium has `responseType: "Cached"` but no explicit hit/miss split |
| Prefetch/Expired counters             | Unbound-specific feature                                               |
| Infrastructure/Key cache counts       | Unbound-specific internals                                             |
| Request list (active/max/avg)         | Unbound-specific internals                                             |

## Decisions

1. **Log format: Resolved.** Technitium Log Exporter App outputs structured JSON
   (JSON Lines). All three sinks (file, syslog, HTTP) use the same format. Alloy
   uses `stage.json` for parsing -- no regex needed.

2. **Block differentiation: Resolved.** `responseType: "Blocked"` directly
   identifies blocked queries. Classification logic: `dns="block"` when
   `responseType == "Blocked"`, `dns="reply"` otherwise.

3. **Response time: Available.** `responseRtt` field (float, milliseconds) is
   present on Recursive queries. Enables avg/percentile response time panels.

4. **Docker Compose: Deferred.** Not needed now. May add
   `contrib/docker-compose/` later as a getting-started convenience.

5. **Two dashboards.**
   - **Default dashboard** (`technitium-dark-dashboard.json`): Uses exporter
     (Prometheus) as primary data source with log-based Loki panels in a
     collapsible row. When Loki datasource is configured and logs are flowing,
     the log row auto-expands and populates. When Loki is absent, the row stays
     collapsed and Prometheus-only panels work standalone.
   - **Logs dashboard** (`technitium-logs-dashboard.json`): In-depth log
     analytics dashboard with filtering, search, and detailed tables. Designed
     for operators who want to drill into DNS query patterns. Same
     collapsible-row pattern for API logs (see decision 6).

6. **Support both Log Exporter and API log collection.**
   - **Log Exporter (syslog or file) is the default.** Alloy receives logs via
     syslog or tails JSON Lines file, parses, and ships to Loki.
   - **API logs are optional.** For environments where the Log Exporter App
     can't be installed. Alloy polls `/api/queryLogs/list`.
   - **Dashboard pattern:** Both dashboards have a collapsible "API Query Logs"
     row. When populated, it auto-expands with additional panels. Two tiers:
     - Log Exporter row: real-time, low-latency, always-on
     - API logs row: fallback for environments without Log Exporter App

7. **EDNS logging: Recommend enabling.** Setting `ebableEdnsLogging: true` adds
   block reasons (EDE/RFC 8914) to log entries. Useful for dashboards showing
   _why_ domains were blocked (blocklist rule, MISP feed, etc.). Document as
   recommended but optional.

## Implementation Order

### Phase 1: Log Exporter Pipeline (unblocked -- log format confirmed)

1. **Write Alloy syslog config** -- `contrib/alloy/syslog.alloy` with
   `stage.json` parsing
2. **Write Alloy file-tail config** -- `contrib/alloy/file-tail.alloy` as
   alternative
3. **Add `loki_datasource` variable** to default dashboard with graceful
   fallback
4. **Add Loki log row** to default dashboard -- 10 panels (Live Queries, Live
   Blocked, Queries Over Time, Client Requests, Top Client Queries, Top Blocked,
   Response Time percentiles, Response Type distribution, Live Clients, Queries
   Distribution)
5. **Create logs dashboard** -- `technitium-logs-dashboard.json` with filtering,
   search, detailed tables, and response time analysis
6. **Write setup docs** -- `contrib/alloy/README.md` covering Log Exporter App
   install + Alloy config for both syslog and file-tail

### Phase 2: API Log Pipeline

1. **Investigate `/api/queryLogs/list` response format** -- Document fields,
   pagination, rate limits
2. **Write Alloy API polling config** -- `contrib/alloy/api-logs.alloy`
3. **Add API logs row** to both dashboards -- collapsible, auto-expands when
   data present
4. **Update docs** -- Add API log setup section to README

### Phase 3: Polish

1. **Test all three deployment tiers** -- Prometheus-only, +Log Exporter, +Log
   Exporter+API
2. **Final documentation pass**
3. **Enable EDNS logging** and add block reason panels if EDE data is useful

## Verification

### Tier 1 (Prometheus only)

1. Default dashboard renders all exporter panels
2. Syslog and API log rows show "no data" or stay collapsed
3. No errors from missing Loki datasource

### Tier 2 (Prometheus + Log Exporter)

1. Technitium Log Exporter sends logs to Alloy (syslog or file tail)
2. Alloy parses JSON and ships to Loki with correct labels (`dns`, `protocol`,
   `response_code`, `response_type`)
3. Structured metadata populated (`client_ip`, `domain`, `record_type`,
   `response_rtt`)
4. `{job="technitium-syslog"}` returns results in Grafana Explore
5. Log row auto-expands in default dashboard with all 10 panels
6. Logs dashboard populates all Log Exporter panels
7. Response time panels show data for Recursive queries
8. API log row stays collapsed

### Tier 3 (Prometheus + Log Exporter + API logs)

1. API log polling populates `{job="technitium-api-logs"}` in Loki
2. API log row auto-expands in both dashboards
3. All panels across both dashboards populated
