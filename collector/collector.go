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
	up                  *prometheus.Desc
	scrapeDuration      *prometheus.Desc
	serverInfo          *prometheus.Desc
	queriesTotal        *prometheus.Desc
	responsesTotal      *prometheus.Desc
	queriesByType       *prometheus.Desc
	blockedTotal        *prometheus.Desc
	blocklistDomains    *prometheus.Desc
	blockedZones        *prometheus.Desc
	allowedZones        *prometheus.Desc
	cacheEntries        *prometheus.Desc
	clients             *prometheus.Desc
	zones               *prometheus.Desc
	queriesByRecordType *prometheus.Desc
	queriesByProtocol   *prometheus.Desc
	uptimeSeconds       *prometheus.Desc

	// Optional top-entry descriptors (nil when disabled).
	topClients        *prometheus.Desc
	topDomains        *prometheus.Desc
	topBlockedDomains *prometheus.Desc

	topEntriesEnabled bool
}

// newDesc creates a prometheus.Desc with the namespace prefix.
func newDesc(subsystem, name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, name),
		help, labels, nil,
	)
}

// NewCollector creates a new Technitium collector.
func NewCollector(client *technitium.Client, logger *slog.Logger, topEntries bool) *Collector {
	c := &Collector{
		client:              client,
		logger:              logger,
		topEntriesEnabled:   topEntries,
		up:                  newDesc("", "up", "Whether the Technitium server is reachable."),
		scrapeDuration:      newDesc("", "scrape_duration_seconds", "Time taken to scrape metrics from Technitium."),
		serverInfo:          newDesc("server", "info", "Technitium DNS server information.", "version", "server_domain"),
		queriesTotal:        newDesc("queries", "total", "Total DNS queries processed."),
		responsesTotal:      newDesc("responses", "total", "DNS responses by response code.", "rcode"),
		queriesByType:       newDesc("queries_by_type", "total", "DNS queries by resolution type.", "type"),
		blockedTotal:        newDesc("blocked_queries", "total", "Total blocked DNS queries."),
		blocklistDomains:    newDesc("", "blocklist_domains", "Number of domains in blocklists."),
		blockedZones:        newDesc("", "blocked_zones", "Number of blocked zones configured."),
		allowedZones:        newDesc("", "allowed_zones", "Number of allowed zones configured."),
		cacheEntries:        newDesc("cache", "entries", "Current number of entries in cache."),
		clients:             newDesc("clients", "total", "Total unique clients seen."),
		zones:               newDesc("", "zones", "Total number of zones."),
		queriesByRecordType: newDesc("queries_by_record_type", "total", "DNS queries by record type (A, AAAA, TXT, etc.).", "record_type"),
		queriesByProtocol:   newDesc("queries_by_protocol", "total", "DNS queries by transport protocol.", "protocol"),
		uptimeSeconds:       newDesc("server", "uptime_seconds", "Technitium DNS server uptime in seconds."),
	}

	if topEntries {
		c.topClients = newDesc("top_clients", "hits", "Top clients by query count.", "client", "rate_limited")
		c.topDomains = newDesc("top_domains", "hits", "Top queried domains by hit count.", "domain")
		c.topBlockedDomains = newDesc("top_blocked_domains", "hits", "Top blocked domains by hit count.", "domain")
	}

	return c
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
	ch <- c.queriesByRecordType
	ch <- c.queriesByProtocol
	ch <- c.uptimeSeconds

	if c.topEntriesEnabled {
		ch <- c.topClients
		ch <- c.topDomains
		ch <- c.topBlockedDomains
	}
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
