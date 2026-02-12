// Package technitium provides a client for the Technitium DNS Server API.
package technitium

// StatsResponse represents the response from /api/dashboard/stats/get.
type StatsResponse struct {
	Response StatsResponseData `json:"response"`
	Status   string            `json:"status"`
	Server   string            `json:"server"`
}

// StatsResponseData contains the stats data from the dashboard endpoint.
type StatsResponseData struct {
	Stats                 Stats       `json:"stats"`
	QueryTypeChartData    ChartData   `json:"queryTypeChartData"`
	ProtocolTypeChartData ChartData   `json:"protocolTypeChartData"`
	TopClients            []TopClient `json:"topClients"`
	TopDomains            []TopEntry  `json:"topDomains"`
	TopBlockedDomains     []TopEntry  `json:"topBlockedDomains"`
}

// ChartData represents Chart.js formatted data from the Technitium dashboard API.
// The labels slice and datasets[0].Data slice are parallel arrays.
type ChartData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

// ChartDataset represents a single dataset in Chart.js formatted data.
type ChartDataset struct {
	Data []int64 `json:"data"`
}

// TopClient represents a top client entry from the dashboard API.
type TopClient struct {
	Name        string `json:"name"`
	Hits        int64  `json:"hits"`
	RateLimited bool   `json:"rateLimited"`
}

// TopEntry represents a top domain entry from the dashboard API.
type TopEntry struct {
	Name string `json:"name"`
	Hits int64  `json:"hits"`
}

// Stats contains the DNS server statistics.
type Stats struct {
	TotalQueries       int64 `json:"totalQueries"`
	TotalNoError       int64 `json:"totalNoError"`
	TotalServerFailure int64 `json:"totalServerFailure"`
	TotalNxDomain      int64 `json:"totalNxDomain"`
	TotalRefused       int64 `json:"totalRefused"`
	TotalAuthoritative int64 `json:"totalAuthoritative"`
	TotalRecursive     int64 `json:"totalRecursive"`
	TotalCached        int64 `json:"totalCached"`
	TotalBlocked       int64 `json:"totalBlocked"`
	TotalDropped       int64 `json:"totalDropped"`
	TotalClients       int64 `json:"totalClients"`
	Zones              int64 `json:"zones"`
	CachedEntries      int64 `json:"cachedEntries"`
	AllowedZones       int64 `json:"allowedZones"`
	BlockedZones       int64 `json:"blockedZones"`
	BlockListZones     int64 `json:"blockListZones"`
}

// SettingsResponse represents the response from /api/settings/get.
type SettingsResponse struct {
	Response SettingsResponseData `json:"response"`
	Status   string               `json:"status"`
}

// SettingsResponseData contains the settings data.
type SettingsResponseData struct {
	Version         string `json:"version"`
	Uptimestamp     string `json:"uptimestamp"`
	DNSServerDomain string `json:"dnsServerDomain"`
}
