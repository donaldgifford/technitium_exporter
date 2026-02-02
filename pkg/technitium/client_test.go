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
