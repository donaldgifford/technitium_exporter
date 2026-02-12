// Package collector provides the Prometheus collector for Technitium DNS Server metrics.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/donaldgifford/technitium_exporter/pkg/technitium"
)

const namespace = "technitium"

// Collector implements the prometheus.Collector interface for Technitium metrics.
type Collector struct {
	client *technitium.Client
	logger *slog.Logger

	// Metric descriptors.
	up               *prometheus.Desc
	scrapeDuration   *prometheus.Desc
	serverInfo       *prometheus.Desc
	queriesTotal     *prometheus.Desc
	responsesTotal   *prometheus.Desc
	queriesByType    *prometheus.Desc
	blockedTotal     *prometheus.Desc
	blocklistDomains *prometheus.Desc
	blockedZones     *prometheus.Desc
	allowedZones     *prometheus.Desc
	cacheEntries     *prometheus.Desc
	clients          *prometheus.Desc
	zones            *prometheus.Desc
}

// NewCollector creates a new Technitium collector.
func NewCollector(client *technitium.Client, logger *slog.Logger) *Collector {
	return &Collector{
		client: client,
		logger: logger,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Whether the Technitium server is reachable.",
			nil, nil,
		),
		scrapeDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
			"Time taken to scrape metrics from Technitium.",
			nil, nil,
		),
		serverInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "server", "info"),
			"Technitium DNS server information.",
			[]string{"version", "server_domain"}, nil,
		),
		queriesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "queries", "total"),
			"Total DNS queries processed.",
			nil, nil,
		),
		responsesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "responses", "total"),
			"DNS responses by response code.",
			[]string{"rcode"}, nil,
		),
		queriesByType: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "queries_by_type", "total"),
			"DNS queries by resolution type.",
			[]string{"type"}, nil,
		),
		blockedTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "blocked_queries", "total"),
			"Total blocked DNS queries.",
			nil, nil,
		),
		blocklistDomains: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "blocklist_domains"),
			"Number of domains in blocklists.",
			nil, nil,
		),
		blockedZones: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "blocked_zones"),
			"Number of blocked zones configured.",
			nil, nil,
		),
		allowedZones: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "allowed_zones"),
			"Number of allowed zones configured.",
			nil, nil,
		),
		cacheEntries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "cache", "entries"),
			"Current number of entries in cache.",
			nil, nil,
		),
		clients: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "clients", "total"),
			"Total unique clients seen.",
			nil, nil,
		),
		zones: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "zones"),
			"Total number of zones.",
			nil, nil,
		),
	}
}

// Describe sends all metric descriptors to the provided channel.
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

// Collect fetches metrics from Technitium and sends them to the provided channel.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	ctx := context.Background()

	// Fetch stats and settings concurrently.
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

	// Stats are required - if they fail, mark as down.
	if statsErr != nil {
		c.logger.Error("Failed to get stats", "err", statsErr)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)

	// Server info - settings endpoint is optional (requires admin permissions).
	// Fall back to server name from stats response if settings unavailable.
	if settingsErr != nil {
		c.logger.Warn("Failed to get settings (requires admin token), using server name from stats", "err", settingsErr)
		ch <- prometheus.MustNewConstMetric(
			c.serverInfo, prometheus.GaugeValue, 1,
			"unknown",
			stats.Server,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			c.serverInfo, prometheus.GaugeValue, 1,
			settings.Response.Version,
			settings.Response.DNSServerDomain,
		)
	}

	s := stats.Response.Stats

	// Query totals.
	ch <- prometheus.MustNewConstMetric(c.queriesTotal, prometheus.CounterValue, float64(s.TotalQueries))

	// Response codes.
	ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalNoError), "noerror")
	ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalServerFailure), "servfail")
	ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalNxDomain), "nxdomain")
	ch <- prometheus.MustNewConstMetric(c.responsesTotal, prometheus.CounterValue, float64(s.TotalRefused), "refused")

	// Query types.
	ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalAuthoritative), "authoritative")
	ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalRecursive), "recursive")
	ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalCached), "cached")
	ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalBlocked), "blocked")
	ch <- prometheus.MustNewConstMetric(c.queriesByType, prometheus.CounterValue, float64(s.TotalDropped), "dropped")

	// Blocking stats.
	ch <- prometheus.MustNewConstMetric(c.blockedTotal, prometheus.CounterValue, float64(s.TotalBlocked))
	ch <- prometheus.MustNewConstMetric(c.blocklistDomains, prometheus.GaugeValue, float64(s.BlockListZones))
	ch <- prometheus.MustNewConstMetric(c.blockedZones, prometheus.GaugeValue, float64(s.BlockedZones))
	ch <- prometheus.MustNewConstMetric(c.allowedZones, prometheus.GaugeValue, float64(s.AllowedZones))

	// Cache, clients, zones.
	ch <- prometheus.MustNewConstMetric(c.cacheEntries, prometheus.GaugeValue, float64(s.CachedEntries))
	ch <- prometheus.MustNewConstMetric(c.clients, prometheus.GaugeValue, float64(s.TotalClients))
	ch <- prometheus.MustNewConstMetric(c.zones, prometheus.GaugeValue, float64(s.Zones))
}
