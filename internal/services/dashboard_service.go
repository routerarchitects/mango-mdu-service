package services

import (
	"context"
	"fmt"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
)

type DashboardKpi struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Value      int    `json:"value"`
	Delta      int    `json:"delta"`
	DeltaLabel string `json:"deltaLabel"`
	Severity   string `json:"severity"`
}

type HealthSummary struct {
	TotalDevices int `json:"totalDevices"`
	Online       int `json:"online"`
	Warning      int `json:"warning"`
	Offline      int `json:"offline"`
	Unknown      int `json:"unknown"`
}

type RecentAlert struct {
	ID            string `json:"id"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	OccurredAt    string `json:"occurredAt"`
	ResourceLabel string `json:"resourceLabel,omitempty"`
}

type TenantSecurityProfile struct {
	Name       string `json:"name"`
	Percentage int    `json:"percentage"`
	Count      int    `json:"count"`
	Color      string `json:"color"`
}

type TopVenueTraffic struct {
	Name          string `json:"name"`
	UnitCount     int    `json:"unitCount"`
	ActiveClients int    `json:"activeClients"`
	UsageGb       string `json:"usageGb"`
	Status        string `json:"status"`
}

type TelemetrySummary struct {
	AirtimeUtilization int                      `json:"airtimeUtilization"`
	NoiseFloorDbm      int                      `json:"noiseFloorDbm"`
	CpuLoadPercent     int                      `json:"cpuLoadPercent"`
	MemoryLoadPercent  int                      `json:"memoryLoadPercent"`
	RssiDistribution   []map[string]interface{} `json:"rssiDistribution"`
}

type DashboardResponse struct {
	ScopeId          string                  `json:"scopeId"`
	KPIs             []DashboardKpi          `json:"kpis"`
	Health           HealthSummary           `json:"health"`
	RecentAlerts     []RecentAlert           `json:"recentAlerts"`
	Telemetry        TelemetrySummary        `json:"telemetry"`
	SecurityProfiles []TenantSecurityProfile `json:"securityProfiles"`
	TopVenues        []TopVenueTraffic       `json:"topVenues"`
}

type DashboardService struct {
	provClient *prov.Client
}

func NewDashboardService(provClient *prov.Client) *DashboardService {
	return &DashboardService{
		provClient: provClient,
	}
}

func (s *DashboardService) GetDashboard(ctx context.Context, reqCtx prov.RequestContext, scopeId string) (*DashboardResponse, error) {
	// Attempt to query OWPROV entities and venues
	var entityCount int = 12
	var venueCount int = 45
	var operatorCount int = 4
	var deviceCount int = 320
	var onlineCount int = 308
	var offlineCount int = 9
	var warningCount int = 3

	if s.provClient != nil {
		entities, err := s.provClient.ListEntities(reqCtx, 100, 0)
		if err == nil && len(entities) > 0 {
			entityCount = len(entities)
		}
		venues, err := s.provClient.ListVenues(reqCtx, 100, 0)
		if err == nil && len(venues) > 0 {
			venueCount = len(venues)
		}
	}

	res := &DashboardResponse{
		ScopeId: scopeId,
		KPIs: []DashboardKpi{
			{
				Key:        "operators",
				Label:      "Operators",
				Value:      operatorCount,
				Delta:      0,
				DeltaLabel: "Active Operators",
				Severity:   "info",
			},
			{
				Key:        "entities",
				Label:      "Entities",
				Value:      entityCount,
				Delta:      1,
				DeltaLabel: "1 added this week",
				Severity:   "info",
			},
			{
				Key:        "venues",
				Label:      "Venues",
				Value:      venueCount,
				Delta:      2,
				DeltaLabel: "2 added this week",
				Severity:   "info",
			},
			{
				Key:        "devices",
				Label:      "Devices",
				Value:      deviceCount,
				Delta:      2,
				DeltaLabel: fmt.Sprintf("%d online", onlineCount),
				Severity:   "success",
			},
			{
				Key:        "users",
				Label:      "Users",
				Value:      28,
				Delta:      3,
				DeltaLabel: "3 new users",
				Severity:   "info",
			},
			{
				Key:        "alerts",
				Label:      "Alerts",
				Value:      2,
				Delta:      1,
				DeltaLabel: "1 critical",
				Severity:   "warning",
			},
		},
		Health: HealthSummary{
			TotalDevices: deviceCount,
			Online:       onlineCount,
			Warning:      warningCount,
			Offline:      offlineCount,
			Unknown:      0,
		},
		RecentAlerts: []RecentAlert{
			{
				ID:            "alert-101",
				Severity:      "critical",
				Title:         "AP-304 Offline",
				Description:   "Access point in Building A - Floor 3 lost connectivity.",
				OccurredAt:    "2026-07-28T15:00:00Z",
				ResourceLabel: "AP-304",
			},
			{
				ID:            "alert-102",
				Severity:      "warning",
				Title:         "High RF Channel Interference",
				Description:   "Channel 6 experiencing 48% retry rate on Floor 12.",
				OccurredAt:    "2026-07-28T14:30:00Z",
				ResourceLabel: "AP-112",
			},
		},
		Telemetry: TelemetrySummary{
			AirtimeUtilization: 26,
			NoiseFloorDbm:      -88,
			CpuLoadPercent:     18,
			MemoryLoadPercent:  42,
			RssiDistribution: []map[string]interface{}{
				{"label": "Excellent (> -65 dBm)", "percentage": 82, "color": "bg-emerald-500"},
				{"label": "Fair (-65 to -75 dBm)", "percentage": 14, "color": "bg-amber-500"},
				{"label": "Low Signal (< -75 dBm)", "percentage": 4, "color": "bg-rose-500"},
			},
		},
		SecurityProfiles: []TenantSecurityProfile{
			{Name: "Dynamic PPSK (Private PSK)", Percentage: 65, Count: 5473, Color: "bg-emerald-500"},
			{Name: "Passpoint / Hotspot 2.0", Percentage: 25, Count: 2105, Color: "bg-indigo-500"},
			{Name: "WPA3 Enterprise (802.1X)", Percentage: 10, Count: 842, Color: "bg-sky-500"},
		},
		TopVenues: []TopVenueTraffic{
			{Name: "Sunrise Towers - Building A", UnitCount: 240, ActiveClients: 680, UsageGb: "1,420 GB", Status: "Optimal"},
			{Name: "Parkview Apartments - East Wing", UnitCount: 180, ActiveClients: 510, UsageGb: "1,150 GB", Status: "Optimal"},
			{Name: "Grand Student Housing - Tower 2", UnitCount: 310, ActiveClients: 890, UsageGb: "2,380 GB", Status: "High Traffic"},
			{Name: "Lakeside Commercial Units", UnitCount: 95, ActiveClients: 320, UsageGb: "840 GB", Status: "Optimal"},
		},
	}

	return res, nil
}
