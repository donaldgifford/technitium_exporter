# technitium_exporter

Prometheus exporter for [Technitium DNS Server](https://technitium.com/dns/).

## Features

- Exposes DNS query statistics, response codes, and cache metrics
- Lightweight, suitable for Raspberry Pi and other ARM64/AMD64 systems
- Systemd service integration for Debian packages
- Follows Prometheus exporter best practices

## Quick Start

### Debian/Ubuntu (Recommended for Raspberry Pi)

Download the latest `.deb` package from
[Releases](https://github.com/donaldgifford/technitium_exporter/releases):

```bash
# Download (replace VERSION and ARCH as needed)
wget https://github.com/donaldgifford/technitium_exporter/releases/download/vVERSION/technitium-exporter_VERSION_arm64.deb

# Install
sudo dpkg -i technitium-exporter_*.deb

# Configure
sudo nano /etc/default/technitium_exporter
```

Edit `/etc/default/technitium_exporter`:

```bash
TECHNITIUM_URL="http://127.0.0.1:5380"
TECHNITIUM_TOKEN="your-api-token-here"
```

Start the service:

```bash
sudo systemctl start technitium_exporter
sudo systemctl status technitium_exporter

# View logs
journalctl -u technitium_exporter -f
```

### Standalone Binary

Download the appropriate binary from
[Releases](https://github.com/donaldgifford/technitium_exporter/releases):

```bash
# Download (replace VERSION, OS, and ARCH as needed)
wget https://github.com/donaldgifford/technitium_exporter/releases/download/vVERSION/technitium_exporter-vVERSION-linux-arm64.tar.gz
tar -xzf technitium_exporter-*.tar.gz

# Run with environment variables
export TECHNITIUM_URL="http://127.0.0.1:5380"
export TECHNITIUM_TOKEN="your-api-token-here"
./technitium_exporter

# Or run with flags
./technitium_exporter \
  --technitium.url=http://127.0.0.1:5380 \
  --technitium.token=your-api-token-here
```

Metrics will be available at `http://localhost:9167/metrics`.

## Configuration

| Flag                   | Environment Variable | Default    | Description                          |
| ---------------------- | -------------------- | ---------- | ------------------------------------ |
| `--technitium.url`     | `TECHNITIUM_URL`     | (required) | Technitium server URL                |
| `--technitium.token`   | `TECHNITIUM_TOKEN`   | (required) | API token                            |
| `--web.listen-address` | `LISTEN_ADDRESS`     | `:9167`    | Address to listen on                 |
| `--web.telemetry-path` | `METRICS_PATH`       | `/metrics` | Path for metrics                     |
| `--scrape.timeout`     | `SCRAPE_TIMEOUT`     | `10s`      | Timeout for API calls                |
| `--log.level`          | -                    | `info`     | Log level (debug, info, warn, error) |

Environment variables take precedence over flags.

## Metrics

| Metric                               | Type    | Description                          |
| ------------------------------------ | ------- | ------------------------------------ |
| `technitium_up`                      | Gauge   | Whether the server is reachable      |
| `technitium_scrape_duration_seconds` | Gauge   | Time taken to scrape                 |
| `technitium_server_info`             | Gauge   | Server info (version, domain labels) |
| `technitium_queries_total`           | Counter | Total DNS queries                    |
| `technitium_responses_total`         | Counter | Responses by rcode                   |
| `technitium_queries_by_type_total`   | Counter | Queries by type                      |
| `technitium_blocked_queries_total`   | Counter | Total blocked queries                |
| `technitium_blocklist_domains`       | Gauge   | Domains in blocklists                |
| `technitium_cache_entries`           | Gauge   | Current cache entries                |
| `technitium_clients_total`           | Gauge   | Unique clients seen                  |
| `technitium_zones`                   | Gauge   | Total zones                          |

## Prometheus Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: "technitium"
    static_configs:
      - targets: ["localhost:9167"]
```

## Building from Source

```bash
# Clone
git clone https://github.com/donaldgifford/technitium_exporter.git
cd technitium_exporter

# Build
make build

# Run tests
make test

# Build packages (requires goreleaser)
goreleaser release --snapshot --clean
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
