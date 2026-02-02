// Package exporter provides HTTP handlers for the Technitium exporter.
package exporter

import (
	"fmt"
	"net/http"
)

const landingPageTemplate = `<!DOCTYPE html>
<html>
<head>
	<title>Technitium Exporter</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 40px; }
		h1 { color: #333; }
		a { color: #0066cc; }
	</style>
</head>
<body>
	<h1>Technitium DNS Exporter</h1>
	<p><a href="%s">Metrics</a></p>
	<p>A Prometheus exporter for <a href="https://technitium.com/dns/">Technitium DNS Server</a>.</p>
</body>
</html>`

// LandingPageHandler returns an HTTP handler that serves a simple landing page.
func LandingPageHandler(metricsPath string) http.HandlerFunc {
	page := []byte(fmt.Sprintf(landingPageTemplate, metricsPath))
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(page); err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	}
}
