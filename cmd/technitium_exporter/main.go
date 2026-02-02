// Package main provides the entry point for the Technitium DNS exporter.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/donaldgifford/technitium_exporter/collector"
	"github.com/donaldgifford/technitium_exporter/config"
	"github.com/donaldgifford/technitium_exporter/exporter"
	"github.com/donaldgifford/technitium_exporter/pkg/technitium"
)

var (
	// Version information set by build flags.
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	app := kingpin.New("technitium_exporter", "Prometheus exporter for Technitium DNS Server.")
	app.HelpFlag.Short('h')
	app.Version(fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate))

	cfg := config.NewConfig(app)

	logLevel := app.Flag("log.level", "Log level (debug, info, warn, error).").
		Default("info").
		Enum("debug", "info", "warn", "error")

	if _, err := app.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing command line: %v\n", err)
		os.Exit(1)
	}

	// Set up structured logging.
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Apply environment variable overrides.
	cfg.ApplyEnvironment()

	// Validate configuration.
	if err := cfg.Validate(); err != nil {
		logger.Error("Configuration error", "err", err)
		os.Exit(1)
	}

	logger.Info("Starting technitium_exporter",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
	)
	logger.Info("Configuration",
		"url", cfg.TechnitiumURL,
		"listen_address", cfg.ListenAddress,
		"metrics_path", cfg.MetricsPath,
	)

	// Create Technitium client.
	client := technitium.NewClient(cfg.TechnitiumURL, cfg.TechnitiumToken, cfg.ScrapeTimeout)

	// Create and register collector.
	coll := collector.NewCollector(client, logger)
	prometheus.MustRegister(coll)

	// Set up HTTP handlers.
	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.Handler())
	mux.HandleFunc("/", exporter.LandingPageHandler(cfg.MetricsPath))

	// Create server with timeouts.
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Handle graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Server shutdown error", "err", err)
		}
	}()

	logger.Info("Listening on", "address", cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
