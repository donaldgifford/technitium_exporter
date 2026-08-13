# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Prometheus exporter for Technitium DNS Server. Exposes DNS server metrics in
Prometheus format.

## Build Commands

Requires Go 1.25.7+ (managed via mise).

```bash
# Build the binary
make build
# or: go build -o build/bin/technitium_exporter ./cmd/technitium_exporter

# Run tests
make test
# or: go test -v -race ./...

# Run tests with coverage
make test-coverage

# Lint code
make lint

# Format code
make fmt

# Run local with test server
make run-local

# Build release packages (snapshot)
make release-local

# Security scanning
make security        # Run govulncheck + trivy
make govulncheck     # Go source-level vulnerability check
make trivy           # Dependency CVE scan (HIGH/CRITICAL)
make syft            # Generate SBOMs (SPDX + CycloneDX) to build/
```

## Running the Exporter

```bash
# Using environment variables
export TECHNITIUM_URL=http://localhost:5380
export TECHNITIUM_TOKEN=your-api-token
./technitium_exporter

# Using flags
./technitium_exporter \
  --technitium.url=http://localhost:5380 \
  --technitium.token=your-api-token \
  --web.listen-address=:9167
```

## Architecture

```
cmd/technitium_exporter/    # Entry point, CLI parsing, HTTP server setup
collector/                  # Prometheus collector (MustNewConstMetric pattern)
config/                     # Configuration handling (flags + env vars)
exporter/                   # HTTP handlers (landing page)
pkg/technitium/             # Technitium API client and response types
deploy/
└── deb/                    # Debian package files (systemd, scripts, copyright)
contrib/
├── grafana/                # Grafana dashboard JSONs (light + dark)
└── prometheus/             # Prometheus alert rules
docs/                       # Planning docs, review findings
scripts/                    # Utility scripts (labels.sh)
```

**Key patterns:**

- Collector uses `MustNewConstMetric` per Prometheus best practices (no direct
  instrumentation)
- Concurrent API calls to stats and settings endpoints
- Graceful shutdown with signal handling
- HTTP server with timeouts (ReadHeaderTimeout, ReadTimeout, WriteTimeout,
  IdleTimeout)
- Tests use `httptest.Server` for HTTP-level mocking (no interface mocking)

## Testing

Tests use `net/http/httptest` to mock HTTP responses at the transport level,
following Prometheus exporter conventions (no mockery/interface mocking):

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Return test JSON responses
}))
client := technitium.NewClient(server.URL, "test-token", timeout)
```

## Metrics Exposed

| Metric                                    | Type    | Description                                                           |
| ----------------------------------------- | ------- | --------------------------------------------------------------------- |
| `technitium_up`                           | Gauge   | Whether the server is reachable                                       |
| `technitium_scrape_duration_seconds`      | Gauge   | Time taken to scrape                                                  |
| `technitium_server_info`                  | Gauge   | Server info (version, domain labels)                                  |
| `technitium_queries_total`                | Counter | Total DNS queries                                                     |
| `technitium_responses_total`              | Counter | Responses by rcode (noerror, servfail, nxdomain, refused)             |
| `technitium_queries_by_type_total`        | Counter | Queries by type (authoritative, recursive, cached, blocked, dropped)  |
| `technitium_blocked_queries_total`        | Counter | Total blocked queries                                                 |
| `technitium_blocklist_domains`            | Gauge   | Domains in blocklists                                                 |
| `technitium_blocked_zones`                | Gauge   | Blocked zones configured                                              |
| `technitium_allowed_zones`                | Gauge   | Allowed zones configured                                              |
| `technitium_cache_entries`                | Gauge   | Current cache entries                                                 |
| `technitium_clients_total`                | Gauge   | Unique clients seen                                                   |
| `technitium_zones`                        | Gauge   | Total zones                                                           |
| `technitium_queries_by_record_type_total` | Counter | Queries by DNS record type (A, AAAA, TXT, etc.)                       |
| `technitium_queries_by_protocol_total`    | Counter | Queries by transport protocol (udp, tcp, etc.)                        |
| `technitium_top_clients_hits`             | Gauge   | Top clients by query count (gated by `--collector.top-entries`)       |
| `technitium_top_domains_hits`             | Gauge   | Top queried domains by hit count (gated by `--collector.top-entries`) |
| `technitium_top_blocked_domains_hits`     | Gauge   | Top blocked domains by hit count (gated by `--collector.top-entries`) |
| `technitium_server_uptime_seconds`        | Gauge   | Server uptime in seconds (requires admin token)                       |

## Code Style

This project follows the Uber Go Style Guide. Key conventions enforced by
`.golangci.yml`:

- Always check returned errors
- Context as first parameter
- Proper error wrapping with `%w`
- Keep functions under 100 lines, cyclomatic complexity under 15
- Close HTTP response bodies
- Use constants for repeated strings (3+ occurrences)
- Avoid deep nesting (max 4 levels)
- Test helpers must call `t.Helper()`

## Technitium API

The exporter uses two endpoints:

- `/api/dashboard/stats/get?token=<token>&type=LastHour` - Query statistics
- `/api/settings/get?token=<token>` - Server version and domain (requires admin
  token)

Note: Settings endpoint requires admin permissions. If unavailable, exporter
falls back to version="unknown" with server name from stats response.

The stats endpoint returns more data than currently parsed. See
`docs/exporter-enhancements-plan.md` for planned additions:

- `queryTypeChartData` - Query counts by DNS record type (A, AAAA, TXT, etc.)
- `protocolTypeChartData` - Query counts by transport protocol (UDP, TCP)
- `topClients` - Top clients with hit counts and rate-limit status
- `topDomains` - Top queried domains
- `topBlockedDomains` - Top blocked domains

### Technitium DNS Apps

The **Log Exporter App** (installed via Technitium admin UI > Apps) provides
structured JSON query logs via syslog, file, and HTTP sinks. This is used for
the Loki integration (see `docs/loki-alloy-integration-plan.md`).

Log format (JSON Lines):

```json
{"clientIp":"10.10.10.91","protocol":"Tcp","responseCode":"NoError","responseRtt":13.498,"responseType":"Recursive","question":{"questionName":"play.google.com","questionType":"A","questionClass":"IN"},"answers":[...],"edns":[],"timestamp":"2026-02-09T15:17:14.193Z"}
```

Key fields: `clientIp`, `protocol` (Udp/Tcp), `responseType`
(Authoritative/Cached/Recursive/Blocked), `responseCode`, `responseRtt` (ms,
Recursive only), `question.questionName`, `question.questionType`.

## Packaging

### Debian Packages

Uses goreleaser with nfpms for Debian package generation:

- **Binary**: `/usr/bin/technitium_exporter`
- **Systemd service**: `/lib/systemd/system/technitium_exporter.service`
- **Config**: `/etc/default/technitium_exporter`
- **User**: Creates `technitium_exporter` system user

Package files in `deploy/deb/`:

- `systemd/` - Service unit file
- `default/` - Environment config template
- `scripts/` - postinstall/preremove scripts
- `copyright` - Debian copyright file
- `lintian-overrides` - Suppress expected warnings

### Changelog

Uses [chglog](https://github.com/goreleaser/chglog) for changelog generation:

```bash
# Format changelog for deb
chglog format --template deb

# Add new entry (after tagging)
chglog add
```

Configuration in `.chglog.yml`, data in `changelog.yml`.

## CI/CD

GitHub Actions workflows in `.github/workflows/`:

- **test.yml**: Lint, test, build, package-lint (lintian), and security scan
  jobs
- **release.yml**: Release automation with goreleaser (includes SBOM generation
  via syft)

The `package-lint` job builds deb packages and runs lintian to verify Debian
policy compliance.

To run lintian locally (macOS via Docker):

```bash
goreleaser release --snapshot --clean --skip=publish,sign
docker run --rm -v ./dist:/dist debian:bookworm-slim \
  sh -c "apt-get update -qq && apt-get install -y -qq lintian >/dev/null 2>&1 && lintian --no-cfg /dist/technitium-exporter_*_amd64.deb"
```

## Security Tooling

Four-tool security stack managed via mise:

| Tool        | Purpose                                          | Make target                    |
| ----------- | ------------------------------------------------ | ------------------------------ |
| gosec       | Go source security patterns                      | Integrated via `golangci-lint` |
| govulncheck | Source-level Go vuln analysis (call-graph aware) | `make govulncheck`             |
| trivy       | Dependency CVE scanning (HIGH/CRITICAL gating)   | `make trivy`                   |
| syft        | SBOM generation (SPDX + CycloneDX)               | `make syft`                    |

- `make security` runs govulncheck + trivy (not syft -- SBOM is artifact
  generation, not a check)
- CI runs govulncheck (`golang/govulncheck-action@v1`) and trivy
  (`aquasecurity/trivy-action@0.33.1`) on every PR
- Release pipeline generates SPDX SBOMs via goreleaser's `sboms` section
  (requires syft installed via `anchore/sbom-action/download-syft@v0`)

## Contrib

Example configurations in `contrib/`:

- `grafana/technitium-dashboard.json` - Grafana dashboard (light)
- `grafana/technitium-dark-dashboard.json` - Grafana dashboard (dark,
  Unbound-inspired)
- `prometheus/alerts.yml` - Prometheus alerting rules

## Planning Docs

Active planning documents in `docs/`:

- `docs/exporter-enhancements-plan.md` - 6 new metric families from existing API
  data (query types, protocol, top clients/domains/blocked, uptime)
- `docs/loki-alloy-integration-plan.md` - Loki + Alloy integration for DNS query
  log analytics (syslog/file/API sources, 10 dashboard panels, 3 deployment
  tiers)
- `docs/security-tooling-plan.md` - Security tooling integration (completed)

## Code Review Findings

Review findings tracked as GitHub issues (#6-#15) and documented in
`docs/review/`:

| Issue | Finding                                | Severity |
| ----- | -------------------------------------- | -------- |
| #6    | Token in query string (API constraint) | HIGH     |
| #7    | `errors.Is` comparison                 | HIGH     |
| #8    | goimports wrong prefix                 | MEDIUM   |
| #9    | Silent SCRAPE_TIMEOUT failure          | MEDIUM   |
| #10   | No response body size limit            | MEDIUM   |
| #11   | URL string concatenation               | MEDIUM   |
| #12   | Hardcoded context.Background()         | LOW      |
| #13   | Duplicate blocked metric               | LOW      |
| #14   | Test helpers missing \*testing.T       | LOW      |
| #15   | Missing config/exporter tests          | LOW      |

## GitHub Labels

Labels are managed via `scripts/labels.sh` and `.github/labeler.yml`. Run
`scripts/labels.sh` to sync labels to the repo. Use `--dry-run` to preview,
`--force` to update existing labels.
