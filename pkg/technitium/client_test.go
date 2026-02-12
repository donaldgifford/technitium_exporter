package technitium

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetStats(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		wantErr        bool
		wantQueries    int64
	}{
		{
			name: "successful response",
			responseBody: `{
				"status": "ok",
				"response": {
					"stats": {
						"totalQueries": 12345,
						"totalNoError": 12000,
						"totalServerFailure": 10,
						"totalNxDomain": 300,
						"totalRefused": 5,
						"totalAuthoritative": 1000,
						"totalRecursive": 10000,
						"totalCached": 8000,
						"totalBlocked": 500,
						"totalDropped": 2,
						"totalClients": 25,
						"zones": 5,
						"cachedEntries": 3000,
						"allowedZones": 0,
						"blockedZones": 3,
						"blockListZones": 150000
					}
				}
			}`,
			responseStatus: http.StatusOK,
			wantErr:        false,
			wantQueries:    12345,
		},
		{
			name:           "server error",
			responseBody:   `{"status": "error"}`,
			responseStatus: http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "api error status",
			responseBody:   `{"status": "error", "errorMessage": "invalid token"}`,
			responseStatus: http.StatusOK,
			wantErr:        true,
		},
		{
			name:           "invalid json",
			responseBody:   `not json`,
			responseStatus: http.StatusOK,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/dashboard/stats/get" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.URL.Query().Get("token") != "test-token" {
					t.Errorf("missing or invalid token")
				}
				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token", 10*time.Second)
			stats, err := client.GetStats(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && stats.Response.Stats.TotalQueries != tt.wantQueries {
				t.Errorf("GetStats() totalQueries = %v, want %v", stats.Response.Stats.TotalQueries, tt.wantQueries)
			}
		})
	}
}

func TestClient_GetSettings(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		wantErr        bool
		wantVersion    string
	}{
		{
			name: "successful response",
			responseBody: `{
				"status": "ok",
				"response": {
					"version": "13.0.1",
					"uptimestamp": "2024-01-15T10:30:00Z",
					"dnsServerDomain": "dns.example.com"
				}
			}`,
			responseStatus: http.StatusOK,
			wantErr:        false,
			wantVersion:    "13.0.1",
		},
		{
			name:           "server error",
			responseBody:   `{"status": "error"}`,
			responseStatus: http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "api error status",
			responseBody:   `{"status": "error", "errorMessage": "unauthorized"}`,
			responseStatus: http.StatusOK,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/settings/get" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token", 10*time.Second)
			settings, err := client.GetSettings(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && settings.Response.Version != tt.wantVersion {
				t.Errorf("GetSettings() version = %v, want %v", settings.Response.Version, tt.wantVersion)
			}
		})
	}
}

func TestClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok", "response": {"stats": {}}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 10*time.Millisecond)
	_, err := client.GetStats(context.Background())

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestStatsResponse_JSON(t *testing.T) {
	jsonData := `{
		"status": "ok",
		"response": {
			"stats": {
				"totalQueries": 100,
				"totalNoError": 90,
				"totalServerFailure": 2,
				"totalNxDomain": 5,
				"totalRefused": 3,
				"totalAuthoritative": 10,
				"totalRecursive": 80,
				"totalCached": 70,
				"totalBlocked": 5,
				"totalDropped": 1,
				"totalClients": 10,
				"zones": 3,
				"cachedEntries": 500,
				"allowedZones": 1,
				"blockedZones": 2,
				"blockListZones": 50000
			}
		}
	}`

	var resp StatsResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("status = %v, want ok", resp.Status)
	}
	if resp.Response.Stats.TotalQueries != 100 {
		t.Errorf("totalQueries = %v, want 100", resp.Response.Stats.TotalQueries)
	}
	if resp.Response.Stats.BlockListZones != 50000 {
		t.Errorf("blockListZones = %v, want 50000", resp.Response.Stats.BlockListZones)
	}
}

func TestStatsResponse_ChartData(t *testing.T) {
	jsonData := `{
		"status": "ok",
		"response": {
			"stats": {
				"totalQueries": 160,
				"totalNoError": 150,
				"totalServerFailure": 0,
				"totalNxDomain": 5,
				"totalRefused": 5,
				"totalAuthoritative": 40,
				"totalRecursive": 100,
				"totalCached": 15,
				"totalBlocked": 3,
				"totalDropped": 2,
				"totalClients": 5,
				"zones": 3,
				"cachedEntries": 200,
				"allowedZones": 0,
				"blockedZones": 1,
				"blockListZones": 50000
			},
			"queryTypeChartData": {
				"labels": ["A", "AAAA", "TXT", "HTTPS", "PTR"],
				"datasets": [{"data": [100, 40, 10, 5, 5]}]
			},
			"protocolTypeChartData": {
				"labels": ["Udp", "Tcp"],
				"datasets": [{"data": [140, 20]}]
			},
			"topClients": [
				{"name": "10.0.0.1", "hits": 80, "rateLimited": false},
				{"name": "10.0.0.2", "hits": 60, "rateLimited": true}
			],
			"topDomains": [
				{"name": "example.com", "hits": 50},
				{"name": "dns.google", "hits": 30}
			],
			"topBlockedDomains": [
				{"name": "ads.example.com", "hits": 3}
			]
		}
	}`

	var resp StatsResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify queryTypeChartData (Chart.js format).
	if got := len(resp.Response.QueryTypeChartData.Labels); got != 5 {
		t.Errorf("len(QueryTypeChartData.Labels) = %v, want 5", got)
	}
	if got := resp.Response.QueryTypeChartData.Labels[0]; got != "A" {
		t.Errorf("QueryTypeChartData.Labels[0] = %v, want A", got)
	}
	if got := resp.Response.QueryTypeChartData.Datasets[0].Data[0]; got != 100 {
		t.Errorf("QueryTypeChartData.Datasets[0].Data[0] = %v, want 100", got)
	}

	// Verify protocolTypeChartData (Chart.js format).
	if got := len(resp.Response.ProtocolTypeChartData.Labels); got != 2 {
		t.Errorf("len(ProtocolTypeChartData.Labels) = %v, want 2", got)
	}
	if got := resp.Response.ProtocolTypeChartData.Datasets[0].Data[0]; got != 140 {
		t.Errorf("ProtocolTypeChartData.Datasets[0].Data[0] = %v, want 140", got)
	}

	// Verify topClients.
	if got := len(resp.Response.TopClients); got != 2 {
		t.Fatalf("len(TopClients) = %v, want 2", got)
	}
	if got := resp.Response.TopClients[0].Name; got != "10.0.0.1" {
		t.Errorf("TopClients[0].Name = %v, want 10.0.0.1", got)
	}
	if got := resp.Response.TopClients[0].Hits; got != 80 {
		t.Errorf("TopClients[0].Hits = %v, want 80", got)
	}
	if resp.Response.TopClients[0].RateLimited {
		t.Error("TopClients[0].RateLimited = true, want false")
	}
	if !resp.Response.TopClients[1].RateLimited {
		t.Error("TopClients[1].RateLimited = false, want true")
	}

	// Verify topDomains.
	if got := len(resp.Response.TopDomains); got != 2 {
		t.Fatalf("len(TopDomains) = %v, want 2", got)
	}
	if got := resp.Response.TopDomains[0].Name; got != "example.com" {
		t.Errorf("TopDomains[0].Name = %v, want example.com", got)
	}
	if got := resp.Response.TopDomains[0].Hits; got != 50 {
		t.Errorf("TopDomains[0].Hits = %v, want 50", got)
	}

	// Verify topBlockedDomains.
	if got := len(resp.Response.TopBlockedDomains); got != 1 {
		t.Fatalf("len(TopBlockedDomains) = %v, want 1", got)
	}
	if got := resp.Response.TopBlockedDomains[0].Name; got != "ads.example.com" {
		t.Errorf("TopBlockedDomains[0].Name = %v, want ads.example.com", got)
	}
	if got := resp.Response.TopBlockedDomains[0].Hits; got != 3 {
		t.Errorf("TopBlockedDomains[0].Hits = %v, want 3", got)
	}
}

func TestStatsResponse_EmptyChartData(t *testing.T) {
	// JSON with only stats -- no chart data fields. Verifies backward compatibility.
	jsonData := `{
		"status": "ok",
		"response": {
			"stats": {
				"totalQueries": 100,
				"totalNoError": 90,
				"totalServerFailure": 2,
				"totalNxDomain": 5,
				"totalRefused": 3,
				"totalAuthoritative": 10,
				"totalRecursive": 80,
				"totalCached": 70,
				"totalBlocked": 5,
				"totalDropped": 1,
				"totalClients": 10,
				"zones": 3,
				"cachedEntries": 500,
				"allowedZones": 1,
				"blockedZones": 2,
				"blockListZones": 50000
			}
		}
	}`

	var resp StatsResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if got := len(resp.Response.QueryTypeChartData.Labels); got != 0 {
		t.Errorf("len(QueryTypeChartData.Labels) = %v, want 0", got)
	}
	if got := len(resp.Response.ProtocolTypeChartData.Labels); got != 0 {
		t.Errorf("len(ProtocolTypeChartData.Labels) = %v, want 0", got)
	}
	if resp.Response.TopClients != nil {
		t.Errorf("TopClients = %v, want nil", resp.Response.TopClients)
	}
	if resp.Response.TopDomains != nil {
		t.Errorf("TopDomains = %v, want nil", resp.Response.TopDomains)
	}
	if resp.Response.TopBlockedDomains != nil {
		t.Errorf("TopBlockedDomains = %v, want nil", resp.Response.TopBlockedDomains)
	}

	// Stats should still parse correctly.
	if resp.Response.Stats.TotalQueries != 100 {
		t.Errorf("totalQueries = %v, want 100", resp.Response.Stats.TotalQueries)
	}
}
