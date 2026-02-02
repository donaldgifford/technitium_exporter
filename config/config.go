// Package config provides configuration handling for the Technitium exporter.
package config

import (
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
)

// Config holds the exporter configuration.
type Config struct {
	TechnitiumURL   string
	TechnitiumToken string
	ListenAddress   string
	MetricsPath     string
	ScrapeTimeout   time.Duration
}

// NewConfig creates a new Config from command-line flags and environment variables.
// Environment variables take precedence over flags.
func NewConfig(app *kingpin.Application) *Config {
	cfg := &Config{}

	app.Flag("technitium.url", "URL of the Technitium DNS server API (e.g., http://localhost:5380).").
		Default("").
		StringVar(&cfg.TechnitiumURL)

	app.Flag("technitium.token", "API token for Technitium DNS server.").
		Default("").
		StringVar(&cfg.TechnitiumToken)

	app.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").
		Default(":9167").
		StringVar(&cfg.ListenAddress)

	app.Flag("web.telemetry-path", "Path under which to expose metrics.").
		Default("/metrics").
		StringVar(&cfg.MetricsPath)

	app.Flag("scrape.timeout", "Timeout for scraping Technitium API.").
		Default("10s").
		DurationVar(&cfg.ScrapeTimeout)

	return cfg
}

// ApplyEnvironment applies environment variable overrides to the configuration.
// Environment variables take precedence over command-line flags.
func (c *Config) ApplyEnvironment() {
	if url := os.Getenv("TECHNITIUM_URL"); url != "" {
		c.TechnitiumURL = url
	}
	if token := os.Getenv("TECHNITIUM_TOKEN"); token != "" {
		c.TechnitiumToken = token
	}
	if addr := os.Getenv("LISTEN_ADDRESS"); addr != "" {
		c.ListenAddress = addr
	}
	if path := os.Getenv("METRICS_PATH"); path != "" {
		c.MetricsPath = path
	}
	if timeout := os.Getenv("SCRAPE_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.ScrapeTimeout = d
		}
	}
}

// Validate checks that required configuration values are set.
func (c *Config) Validate() error {
	if c.TechnitiumURL == "" {
		return ErrMissingURL
	}
	if c.TechnitiumToken == "" {
		return ErrMissingToken
	}
	return nil
}
