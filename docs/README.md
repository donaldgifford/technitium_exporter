# Technitium DNS Prometheus Exporter

## Problem

Existing Technitium exporters are abandoned, poorly maintained, or written in
suboptimal languages. We need a reliable, lightweight Prometheus exporter for
Technitium DNS Server.

---

## Goals

1. **Prometheus metrics** - Expose Technitium stats in Prometheus format
2. **Grafana dashboard** - Pre-built dashboard for common DNS metrics
3. **Lightweight** - Minimal resource usage, suitable for Raspberry Pi
4. **Multi-platform** - ARM64 (Pi, Mac M-series) and AMD64
5. **Simple deployment** - Docker container or standalone binary
6. **MVP first** - Start small, iterate

---

## Non-Goals (for MVP)

- Log shipping to Loki (future project with Alloy)
- Query-level metrics (individual domain stats)
- Multi-instance discovery
- Alerting rules (can add later)
- Helm Chart

---

## Tech Stack

| Component | Choice                        | Rationale                                     |
| --------- | ----------------------------- | --------------------------------------------- |
| Language  | **Go**                        | Single binary, low memory, easy cross-compile |
| Metrics   | `prometheus/client_golang`    | Standard library                              |
| HTTP      | `net/http` (stdlib)           | No external deps needed                       |
| Config    | Environment variables + flags | Simple, 12-factor                             |
| Build     | GoReleaser                    | Multi-arch binaries + Docker                  |

---

## Technitium API

Base URL: `http://<host>:5380/api`

Authentication: API token (passed as `?token=<token>` or header)

### Relevant Endpoints

| Endpoint                   | Data                                         | MVP                |
| -------------------------- | -------------------------------------------- | ------------------ |
| `/api/dashboard/stats/get` | Main stats (queries, blocked, clients, etc.) | Yes                |
| `/api/settings/get`        | Server settings, version                     | Yes                |
| `/api/cache/list`          | Cache stats                                  | No                 |
| `/api/zones/list`          | Zone information                             | Later              |
| `/api/logs/query`          | Query logs                                   | No (Alloy project) |

### Dashboard Stats Response (key fields)

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
      "totalClients": 25,
      "zones": 5,
      "cachedEntries": 5000,
      "allowedZones": 0,
      "blockedZones": 3,
      "blockListZones": 150000
    },
    "mainChartData": { ... },
    "queryResponseChartData": { ... },
    "queryTypeChartData": { ... },
    "protocolTypeChartData": { ... }
  }
}
```

---

## Prometheus Metrics (MVP)

### Server Info

```prometheus
# HELP technitium_info Server information
# TYPE technitium_info gauge
technitium_info{version="13.0", server="dns03.fartlab.dev"} 1
```

### Query Counters

```prometheus
# HELP technitium_queries_total Total DNS queries
# TYPE technitium_queries_total counter
technitium_queries_total 123456

# HELP technitium_queries_by_response_total Queries by response code
# TYPE technitium_queries_by_response_total counter
technitium_queries_by_response_total{response="noerror"} 120000
technitium_queries_by_response_total{response="servfail"} 100
technitium_queries_by_response_total{response="nxdomain"} 3000
technitium_queries_by_response_total{response="refused"} 50

# HELP technitium_queries_by_type_total Queries by resolution type
# TYPE technitium_queries_by_type_total counter
technitium_queries_by_type_total{type="authoritative"} 5000
technitium_queries_by_type_total{type="recursive"} 118000
technitium_queries_by_type_total{type="cached"} 80000
technitium_queries_by_type_total{type="blocked"} 10000
```

### Blocking Stats

```prometheus
# HELP technitium_blocked_total Total blocked queries
# TYPE technitium_blocked_total counter
technitium_blocked_total 10000

# HELP technitium_blocklist_domains Total domains in blocklists
# TYPE technitium_blocklist_domains gauge
technitium_blocklist_domains 150000
```

### Cache Stats

```prometheus
# HELP technitium_cache_entries Current cache entries
# TYPE technitium_cache_entries gauge
technitium_cache_entries 5000
```

### Client Stats

```prometheus
# HELP technitium_clients_total Total unique clients
# TYPE technitium_clients_total gauge
technitium_clients_total 25
```

### Zone Stats

```prometheus
# HELP technitium_zones_total Total zones
# TYPE technitium_zones_total gauge
technitium_zones_total 5
```

### Exporter Meta

```prometheus
# HELP technitium_exporter_scrape_duration_seconds Time taken for scrape
# TYPE technitium_exporter_scrape_duration_seconds gauge
technitium_exporter_scrape_duration_seconds 0.023

# HELP technitium_exporter_scrape_success Whether scrape was successful
# TYPE technitium_exporter_scrape_success gauge
technitium_exporter_scrape_success 1
```

---

## Configuration

### Environment Variables

| Variable           | Required | Default    | Description                                              |
| ------------------ | -------- | ---------- | -------------------------------------------------------- |
| `TECHNITIUM_URL`   | Yes      | -          | Technitium server URL (e.g., `http://10.10.10.194:5380`) |
| `TECHNITIUM_TOKEN` | Yes      | -          | API token from Technitium                                |
| `LISTEN_ADDRESS`   | No       | `:9167`    | Address to listen on                                     |
| `METRICS_PATH`     | No       | `/metrics` | Path for metrics endpoint                                |
| `LOG_LEVEL`        | No       | `info`     | Log level (debug, info, warn, error)                     |
| `SCRAPE_TIMEOUT`   | No       | `10s`      | Timeout for Technitium API calls                         |

### Command Line Flags

```bash
technitium-exporter \
  --technitium.url=http://10.10.10.194:5380 \
  --technitium.token=xxx \
  --web.listen-address=:9167 \
  --web.metrics-path=/metrics \
  --log.level=info
```

Environment variables take precedence over flags.

---

## Deployment

### Docker

```dockerfile
FROM gcr.io/distroless/static:nonroot
COPY technitium-exporter /technitium-exporter
USER nonroot:nonroot
EXPOSE 9167
ENTRYPOINT ["/technitium-exporter"]
```

```bash
docker run -d \
  -e TECHNITIUM_URL=http://10.10.10.194:5380 \
  -e TECHNITIUM_TOKEN=xxx \
  -p 9167:9167 \
  ghcr.io/donaldgifford/technitium-exporter:latest
```

### Binary (Raspberry Pi)

```bash
# Download
curl -LO https://github.com/donaldgifford/technitium-exporter/releases/latest/download/technitium-exporter-linux-arm64

# Install
chmod +x technitium-exporter-linux-arm64
sudo mv technitium-exporter-linux-arm64 /usr/local/bin/technitium-exporter

# Systemd service
sudo tee /etc/systemd/system/technitium-exporter.service << 'EOF'
[Unit]
Description=Technitium DNS Exporter
After=network.target

[Service]
Type=simple
Environment="TECHNITIUM_URL=http://127.0.0.1:5380"
Environment="TECHNITIUM_TOKEN=xxx"
ExecStart=/usr/local/bin/technitium-exporter
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable --now technitium-exporter
```

### Kubernetes (Sidecar or Standalone)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: technitium-exporter
spec:
  template:
    spec:
      containers:
        - name: exporter
          image: ghcr.io/donaldgifford/technitium-exporter:latest
          env:
            - name: TECHNITIUM_URL
              value: "http://10.10.10.194:5380"
            - name: TECHNITIUM_TOKEN
              valueFrom:
                secretKeyRef:
                  name: technitium-api-token
                  key: token
          ports:
            - containerPort: 9167
```

---

## Build Matrix

| OS     | Arch  | Binary Name                              |
| ------ | ----- | ---------------------------------------- |
| Linux  | amd64 | `technitium-exporter-linux-amd64`        |
| Linux  | arm64 | `technitium-exporter-linux-arm64`        |
| Darwin | arm64 | `technitium-exporter-darwin-arm64`       |
| Docker | multi | `ghcr.io/.../technitium-exporter:latest` |

Build with GoReleaser for consistent multi-arch releases.

---

## Grafana Dashboard

### Panels (MVP)

1. **Overview Row**
   - Total Queries (stat)
   - Blocked Queries (stat)
   - Block Rate % (stat)
   - Unique Clients (stat)

2. **Query Trends Row**
   - Queries over time (time series)
   - Blocked over time (time series)

3. **Response Breakdown Row**
   - Response codes pie chart (NoError, NXDomain, ServFail, Refused)
   - Query types pie chart (Authoritative, Recursive, Cached, Blocked)

4. **Cache Row**
   - Cache entries (gauge)
   - Cache hit rate (if calculable)

5. **Server Info Row**
   - Version, uptime, zones count

### Dashboard JSON

Export as JSON, store in repo at `grafana/dashboards/technitium.json`

Can be auto-provisioned via Grafana dashboard ConfigMap.

---

## Project Structure

```
technitium-exporter/
├── main.go                 # Entry point
├── collector/
│   └── collector.go        # Prometheus collector implementation
├── technitium/
│   └── client.go           # Technitium API client
├── Dockerfile
├── .goreleaser.yml
├── grafana/
│   └── dashboards/
│       └── technitium.json
├── README.md
└── examples/
    ├── docker-compose.yml
    ├── kubernetes.yaml
    └── systemd/
        └── technitium-exporter.service
```

---

## MVP Milestones

### M1: Basic Exporter

- [ ] Technitium API client (dashboard stats endpoint)
- [ ] Prometheus collector with core metrics
- [ ] CLI flags and env var config
- [ ] Basic logging
- [ ] Local testing

### M2: Packaging

- [ ] Dockerfile (distroless)
- [ ] GoReleaser config
- [ ] GitHub Actions for releases
- [ ] Multi-arch builds (linux/amd64, linux/arm64, darwin/arm64)

### M3: Dashboard

- [ ] Grafana dashboard JSON
- [ ] Dashboard provisioning example
- [ ] Screenshots for README

### M4: Documentation

- [ ] README with usage examples
- [ ] Systemd service example
- [ ] Kubernetes deployment example
- [ ] Docker Compose example

---

## Future Enhancements (Post-MVP)

- [ ] Per-zone metrics
- [ ] Top blocked domains metric
- [ ] Top clients metric
- [ ] Query latency histogram (if API provides)
- [ ] Health check endpoint (`/healthz`, `/readyz`)
- [ ] Multiple Technitium instance support
- [ ] Alloy integration for log shipping
- [ ] Alerting rule examples

---

## References

- [Technitium API Docs](https://github.com/TechnitiumSoftware/DnsServer/blob/master/APIDOCS.md)
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [GoReleaser](https://goreleaser.com/)
- [Distroless Images](https://github.com/GoogleContainerTools/distroless)
