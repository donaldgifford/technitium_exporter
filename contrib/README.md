# Monitoring

Example configurations for monitoring Technitium DNS with Prometheus and
Grafana.

## Prometheus Alerts

`prometheus/alerts.yml` contains alert rules for:

- **TechnitiumExporterDown** - Exporter unreachable
- **TechnitiumServerDown** - DNS server API unreachable
- **TechnitiumHighServerFailureRate** - High SERVFAIL rate (>5%)
- **TechnitiumHighNXDomainRate** - High NXDOMAIN rate (>30%)
- **TechnitiumHighRefusedRate** - High REFUSED rate (>10%)
- **TechnitiumHighBlockRate** - High block rate (>50%)
- **TechnitiumBlocklistEmpty** - No domains in blocklist
- **TechnitiumLowCacheHitRate** - Low cache hit rate (<20%)
- **TechnitiumSlowScrape** - Slow API scrape (>5s)

### Usage

Add to your Prometheus configuration:

```yaml
rule_files:
  - /path/to/alerts.yml
```

## Grafana Dashboard

`grafana/technitium-dashboard.json` provides:

- **Overview**: Status, total queries, blocked queries, clients, cache entries,
  zones, blocklist size, block rate
- **Query Trends**: Query rate and queries by type over time
- **Response Codes**: Pie chart distribution and time series of response codes
  (noerror, servfail, nxdomain, refused)
- **Server Info**: Scrape duration, server version, server domain, zone
  overrides

### Import

1. In Grafana, go to **Dashboards → Import**
2. Upload `technitium-dashboard.json` or paste its contents
3. Select your Prometheus datasource
4. Click **Import**

The dashboard includes variables for datasource and instance selection.
