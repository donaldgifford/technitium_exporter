package collector

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/donaldgifford/technitium_exporter/pkg/technitium"
)

const testTimeout = 5 * time.Second

// newTestLogger creates a logger that discards output for testing.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// realWorldStatsJSON returns stats JSON based on actual Technitium server response.
func realWorldStatsJSON() string {
	return `{
		"status": "ok",
		"server": "dns03.fartlab.dev",
		"response": {
			"stats": {
				"totalQueries": 72,
				"totalNoError": 72,
				"totalServerFailure": 0,
				"totalNxDomain": 0,
				"totalRefused": 0,
				"totalAuthoritative": 72,
				"totalRecursive": 0,
				"totalCached": 0,
				"totalBlocked": 0,
				"totalDropped": 0,
				"totalClients": 2,
				"zones": 8,
				"cachedEntries": 35,
				"allowedZones": 0,
				"blockedZones": 0,
				"blockListZones": 214040
			},
			"queryTypeChartData": {"A": 40, "AAAA": 25, "PTR": 5, "TXT": 2},
			"protocolTypeChartData": {"UDP": 65, "TCP": 7},
			"topClients": [
				{"name": "10.10.11.18", "hits": 50, "rateLimited": false},
				{"name": "10.10.11.1", "hits": 22, "rateLimited": false}
			],
			"topDomains": [
				{"name": "dns03.fartlab.dev", "hits": 30},
				{"name": "example.com", "hits": 15}
			],
			"topBlockedDomains": [
				{"name": "ads.example.com", "hits": 8}
			]
		}
	}`
}

// highTrafficStatsJSON returns stats simulating a busy DNS server.
func highTrafficStatsJSON() string {
	return `{
		"status": "ok",
		"server": "dns-prod.example.com",
		"response": {
			"stats": {
				"totalQueries": 1523456,
				"totalNoError": 1450000,
				"totalServerFailure": 1234,
				"totalNxDomain": 52000,
				"totalRefused": 222,
				"totalAuthoritative": 125000,
				"totalRecursive": 1100000,
				"totalCached": 850000,
				"totalBlocked": 48000,
				"totalDropped": 156,
				"totalClients": 347,
				"zones": 42,
				"cachedEntries": 125000,
				"allowedZones": 5,
				"blockedZones": 12,
				"blockListZones": 500000
			},
			"queryTypeChartData": {"A": 800000, "AAAA": 500000, "TXT": 100000, "HTTPS": 50000, "PTR": 30000, "SRV": 2000},
			"protocolTypeChartData": {"UDP": 1400000, "TCP": 123456},
			"topClients": [
				{"name": "10.0.0.1", "hits": 250000, "rateLimited": false},
				{"name": "10.0.0.2", "hits": 180000, "rateLimited": true}
			],
			"topDomains": [
				{"name": "api.github.com", "hits": 50000},
				{"name": "dns.google", "hits": 30000}
			],
			"topBlockedDomains": [
				{"name": "stats.grafana.org", "hits": 25000},
				{"name": "telemetry.example.com", "hits": 15000}
			]
		}
	}`
}

// realWorldSettingsJSON returns settings based on actual Technitium server response.
func realWorldSettingsJSON() string {
	return `{
		"status": "ok",
		"response": {
			"version": "13.0.2",
			"uptimestamp": "2024-01-15T10:30:00Z",
			"dnsServerDomain": "dns03.fartlab.dev"
		}
	}`
}

// errorResponseJSON returns an error response from the API.
func errorResponseJSON() string {
	return `{"status": "error", "errorMessage": "Access denied"}`
}

// newTestServer creates an httptest server that handles stats and settings endpoints.
// statsCode allows testing HTTP-level errors for the stats endpoint.
// Settings endpoint always returns HTTP 200 (API-level errors are tested via JSON status field).
func newTestServer(statsJSON, settingsJSON string, statsCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/dashboard/stats/get"):
			w.WriteHeader(statsCode)
			_, _ = w.Write([]byte(statsJSON))
		case strings.Contains(r.URL.Path, "/api/settings/get"):
			_, _ = w.Write([]byte(settingsJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestCollector_Collect_RealWorldData(t *testing.T) {
	server := newTestServer(realWorldStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	// Collect metrics.
	ch := make(chan prometheus.Metric, 100)
	coll.Collect(ch)
	close(ch)

	// Count metrics.
	metricCount := 0
	for range ch {
		metricCount++
	}

	// We expect: 20 original + 4 queryTypeChartData + 2 protocolTypeChartData +
	// 2 topClients + 2 topDomains + 1 topBlockedDomains + 1 uptimeSeconds = 32.
	expectedMetrics := 32
	if metricCount != expectedMetrics {
		t.Errorf("expected %d metrics, got %d", expectedMetrics, metricCount)
	}
}

func TestCollector_Collect_HighTrafficServer(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	// Test specific metric values using testutil.
	expected := `
		# HELP technitium_queries_total Total DNS queries processed.
		# TYPE technitium_queries_total counter
		technitium_queries_total 1.523456e+06
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_queries_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_queries_total: %v", err)
	}
}

func TestCollector_Collect_SettingsAccessDenied(t *testing.T) {
	server := newTestServer(realWorldStatsJSON(), errorResponseJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	// Should still work with version="unknown".
	expected := `
		# HELP technitium_up Whether the Technitium server is reachable.
		# TYPE technitium_up gauge
		technitium_up 1
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_up"); err != nil {
		t.Errorf("unexpected metrics for technitium_up: %v", err)
	}

	// Server info should have unknown version but real server name.
	expected = `
		# HELP technitium_server_info Technitium DNS server information.
		# TYPE technitium_server_info gauge
		technitium_server_info{server_domain="dns03.fartlab.dev",version="unknown"} 1
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_server_info"); err != nil {
		t.Errorf("unexpected metrics for technitium_server_info: %v", err)
	}
}

func TestCollector_Collect_StatsError(t *testing.T) {
	server := newTestServer(errorResponseJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	ch := make(chan prometheus.Metric, 100)
	coll.Collect(ch)
	close(ch)

	// Should have up=0 and scrape_duration metrics only.
	metricCount := 0
	for range ch {
		metricCount++
	}

	if metricCount != 2 {
		t.Errorf("expected 2 metrics on stats error, got %d", metricCount)
	}
}

func TestCollector_Collect_StatsHTTPError(t *testing.T) {
	server := newTestServer("", realWorldSettingsJSON(), http.StatusInternalServerError)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_up Whether the Technitium server is reachable.
		# TYPE technitium_up gauge
		technitium_up 0
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_up"); err != nil {
		t.Errorf("unexpected metrics for technitium_up: %v", err)
	}
}

func TestCollector_Collect_BothEndpointsFail(t *testing.T) {
	server := newTestServer(errorResponseJSON(), errorResponseJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_up Whether the Technitium server is reachable.
		# TYPE technitium_up gauge
		technitium_up 0
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_up"); err != nil {
		t.Errorf("unexpected metrics for technitium_up: %v", err)
	}
}

func TestCollector_Collect_ServerUnreachable(t *testing.T) {
	// Use a URL that will fail to connect.
	client := technitium.NewClient("http://127.0.0.1:1", "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_up Whether the Technitium server is reachable.
		# TYPE technitium_up gauge
		technitium_up 0
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_up"); err != nil {
		t.Errorf("unexpected metrics for technitium_up: %v", err)
	}
}

func TestCollector_Describe(t *testing.T) {
	client := technitium.NewClient("http://localhost", "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	ch := make(chan *prometheus.Desc, 100)
	coll.Describe(ch)
	close(ch)

	descCount := 0
	for range ch {
		descCount++
	}

	// We expect 19 metric descriptors (13 original + 3 chart data + 3 top entries).
	expectedDescs := 19
	if descCount != expectedDescs {
		t.Errorf("expected %d descriptors, got %d", expectedDescs, descCount)
	}
}

func TestCollector_MetricValues_RealWorld(t *testing.T) {
	server := newTestServer(realWorldStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	// Test queries total.
	expected := `
		# HELP technitium_queries_total Total DNS queries processed.
		# TYPE technitium_queries_total counter
		technitium_queries_total 72
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_queries_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_queries_total: %v", err)
	}

	// Test zones.
	expected = `
		# HELP technitium_zones Total number of zones.
		# TYPE technitium_zones gauge
		technitium_zones 8
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_zones"); err != nil {
		t.Errorf("unexpected metrics for technitium_zones: %v", err)
	}

	// Test blocklist domains.
	expected = `
		# HELP technitium_blocklist_domains Number of domains in blocklists.
		# TYPE technitium_blocklist_domains gauge
		technitium_blocklist_domains 214040
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_blocklist_domains"); err != nil {
		t.Errorf("unexpected metrics for technitium_blocklist_domains: %v", err)
	}

	// Test server info with version.
	expected = `
		# HELP technitium_server_info Technitium DNS server information.
		# TYPE technitium_server_info gauge
		technitium_server_info{server_domain="dns03.fartlab.dev",version="13.0.2"} 1
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_server_info"); err != nil {
		t.Errorf("unexpected metrics for technitium_server_info: %v", err)
	}
}

func TestCollector_ResponseCodes(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_responses_total DNS responses by response code.
		# TYPE technitium_responses_total counter
		technitium_responses_total{rcode="noerror"} 1.45e+06
		technitium_responses_total{rcode="nxdomain"} 52000
		technitium_responses_total{rcode="refused"} 222
		technitium_responses_total{rcode="servfail"} 1234
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_responses_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_responses_total: %v", err)
	}
}

func TestCollector_QueryTypes(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_queries_by_type_total DNS queries by resolution type.
		# TYPE technitium_queries_by_type_total counter
		technitium_queries_by_type_total{type="authoritative"} 125000
		technitium_queries_by_type_total{type="blocked"} 48000
		technitium_queries_by_type_total{type="cached"} 850000
		technitium_queries_by_type_total{type="dropped"} 156
		technitium_queries_by_type_total{type="recursive"} 1.1e+06
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_queries_by_type_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_queries_by_type_total: %v", err)
	}
}

// TestCollector_JSONParsing verifies that the collector correctly parses JSON responses.
func TestCollector_JSONParsing(t *testing.T) {
	// Create a custom response with specific values to verify parsing.
	statsJSON := `{
		"status": "ok",
		"server": "test-server",
		"response": {
			"stats": {
				"totalQueries": 12345,
				"totalNoError": 12000,
				"totalServerFailure": 100,
				"totalNxDomain": 200,
				"totalRefused": 45,
				"totalAuthoritative": 5000,
				"totalRecursive": 6000,
				"totalCached": 1000,
				"totalBlocked": 300,
				"totalDropped": 45,
				"totalClients": 50,
				"zones": 10,
				"cachedEntries": 5000,
				"allowedZones": 2,
				"blockedZones": 3,
				"blockListZones": 100000
			}
		}
	}`

	server := newTestServer(statsJSON, realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	// Verify specific parsed values.
	expected := `
		# HELP technitium_queries_total Total DNS queries processed.
		# TYPE technitium_queries_total counter
		technitium_queries_total 12345
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_queries_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_queries_total: %v", err)
	}

	expected = `
		# HELP technitium_clients_total Total unique clients seen.
		# TYPE technitium_clients_total gauge
		technitium_clients_total 50
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_clients_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_clients_total: %v", err)
	}
}

func TestCollector_QueriesByRecordType(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_queries_by_record_type_total DNS queries by record type (A, AAAA, TXT, etc.).
		# TYPE technitium_queries_by_record_type_total counter
		technitium_queries_by_record_type_total{record_type="A"} 800000
		technitium_queries_by_record_type_total{record_type="AAAA"} 500000
		technitium_queries_by_record_type_total{record_type="HTTPS"} 50000
		technitium_queries_by_record_type_total{record_type="PTR"} 30000
		technitium_queries_by_record_type_total{record_type="SRV"} 2000
		technitium_queries_by_record_type_total{record_type="TXT"} 100000
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_queries_by_record_type_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_queries_by_record_type_total: %v", err)
	}
}

func TestCollector_QueriesByProtocol(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_queries_by_protocol_total DNS queries by transport protocol.
		# TYPE technitium_queries_by_protocol_total counter
		technitium_queries_by_protocol_total{protocol="tcp"} 123456
		technitium_queries_by_protocol_total{protocol="udp"} 1.4e+06
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_queries_by_protocol_total"); err != nil {
		t.Errorf("unexpected metrics for technitium_queries_by_protocol_total: %v", err)
	}
}

func TestCollector_TopClients(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_top_clients_hits Top clients by query count.
		# TYPE technitium_top_clients_hits gauge
		technitium_top_clients_hits{client="10.0.0.1",rate_limited="false"} 250000
		technitium_top_clients_hits{client="10.0.0.2",rate_limited="true"} 180000
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_top_clients_hits"); err != nil {
		t.Errorf("unexpected metrics for technitium_top_clients_hits: %v", err)
	}
}

func TestCollector_TopDomains(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_top_domains_hits Top queried domains by hit count.
		# TYPE technitium_top_domains_hits gauge
		technitium_top_domains_hits{domain="api.github.com"} 50000
		technitium_top_domains_hits{domain="dns.google"} 30000
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_top_domains_hits"); err != nil {
		t.Errorf("unexpected metrics for technitium_top_domains_hits: %v", err)
	}
}

func TestCollector_TopBlockedDomains(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	expected := `
		# HELP technitium_top_blocked_domains_hits Top blocked domains by hit count.
		# TYPE technitium_top_blocked_domains_hits gauge
		technitium_top_blocked_domains_hits{domain="stats.grafana.org"} 25000
		technitium_top_blocked_domains_hits{domain="telemetry.example.com"} 15000
	`
	if err := testutil.CollectAndCompare(coll, strings.NewReader(expected), "technitium_top_blocked_domains_hits"); err != nil {
		t.Errorf("unexpected metrics for technitium_top_blocked_domains_hits: %v", err)
	}
}

func TestCollector_TopEntriesDisabled(t *testing.T) {
	server := newTestServer(highTrafficStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), false)

	// Verify descriptor count is 16 (19 - 3 top-entry descriptors).
	descCh := make(chan *prometheus.Desc, 100)
	coll.Describe(descCh)
	close(descCh)

	descCount := 0
	for range descCh {
		descCount++
	}
	if descCount != 16 {
		t.Errorf("expected 16 descriptors with top entries disabled, got %d", descCount)
	}

	// Verify top_clients, top_domains, top_blocked_domains are NOT emitted.
	metricCount := testutil.CollectAndCount(coll, "technitium_top_clients_hits")
	if metricCount != 0 {
		t.Errorf("expected 0 top_clients metrics when disabled, got %d", metricCount)
	}
	metricCount = testutil.CollectAndCount(coll, "technitium_top_domains_hits")
	if metricCount != 0 {
		t.Errorf("expected 0 top_domains metrics when disabled, got %d", metricCount)
	}
	metricCount = testutil.CollectAndCount(coll, "technitium_top_blocked_domains_hits")
	if metricCount != 0 {
		t.Errorf("expected 0 top_blocked_domains metrics when disabled, got %d", metricCount)
	}

	// Verify record_type and protocol metrics ARE still emitted.
	metricCount = testutil.CollectAndCount(coll, "technitium_queries_by_record_type_total")
	if metricCount == 0 {
		t.Error("expected queries_by_record_type metrics when top entries disabled, got 0")
	}
	metricCount = testutil.CollectAndCount(coll, "technitium_queries_by_protocol_total")
	if metricCount == 0 {
		t.Error("expected queries_by_protocol metrics when top entries disabled, got 0")
	}
}

func TestCollector_UptimeSeconds(t *testing.T) {
	server := newTestServer(realWorldStatsJSON(), realWorldSettingsJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	// Uptime should be a positive value (settings has uptimestamp "2024-01-15T10:30:00Z").
	metricCount := testutil.CollectAndCount(coll, "technitium_server_uptime_seconds")
	if metricCount != 1 {
		t.Errorf("expected 1 uptime metric, got %d", metricCount)
	}
}

func TestCollector_UptimeSeconds_NoSettings(t *testing.T) {
	server := newTestServer(realWorldStatsJSON(), errorResponseJSON(), http.StatusOK)
	defer server.Close()

	client := technitium.NewClient(server.URL, "test-token", testTimeout)
	coll := NewCollector(client, newTestLogger(), true)

	// Uptime should NOT be emitted when settings endpoint fails.
	metricCount := testutil.CollectAndCount(coll, "technitium_server_uptime_seconds")
	if metricCount != 0 {
		t.Errorf("expected 0 uptime metrics when settings fail, got %d", metricCount)
	}
}

// TestStatsResponse_Unmarshal verifies JSON unmarshaling works correctly.
func TestStatsResponse_Unmarshal(t *testing.T) {
	jsonData := realWorldStatsJSON()
	var resp technitium.StatsResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal stats response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
	if resp.Server != "dns03.fartlab.dev" {
		t.Errorf("expected server 'dns03.fartlab.dev', got '%s'", resp.Server)
	}
	if resp.Response.Stats.TotalQueries != 72 {
		t.Errorf("expected totalQueries 72, got %d", resp.Response.Stats.TotalQueries)
	}
	if resp.Response.Stats.BlockListZones != 214040 {
		t.Errorf("expected blockListZones 214040, got %d", resp.Response.Stats.BlockListZones)
	}

	// Verify chart data fields parse correctly.
	if got := resp.Response.QueryTypeChartData["A"]; got != 40 {
		t.Errorf("QueryTypeChartData[A] = %v, want 40", got)
	}
	if got := len(resp.Response.TopClients); got != 2 {
		t.Errorf("len(TopClients) = %v, want 2", got)
	}
	if got := len(resp.Response.TopBlockedDomains); got != 1 {
		t.Errorf("len(TopBlockedDomains) = %v, want 1", got)
	}
}
