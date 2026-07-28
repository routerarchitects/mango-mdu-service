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
	if s.provClient == nil {
		return nil, fmt.Errorf("owprov service discovery client is uninitialized")
	}

	// Fetch real entities from OWPROV (fail fast on downstream error)
	entities, err := s.provClient.ListEntities(reqCtx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("owprov ListEntities error: %w", err)
	}

	// Fetch real venues from OWPROV (fail fast on downstream error)
	venues, err := s.provClient.ListVenues(reqCtx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("owprov ListVenues error: %w", err)
	}

	entityCount := len(entities)
	venueCount := len(venues)
	deviceCount := 0

	// Count devices across all fetched venues
	for _, v := range venues {
		deviceCount += len(v.Devices)
	}

	topVenuesList := make([]TopVenueTraffic, 0, len(venues))
	for idx, v := range venues {
		if idx >= 5 {
			break
		}
		topVenuesList = append(topVenuesList, TopVenueTraffic{
			Name:          v.Info.Name,
			UnitCount:     len(v.Devices),
			ActiveClients: 0,
			UsageGb:       "0 GB",
			Status:        "Optimal",
		})
	}

	res := &DashboardResponse{
		ScopeId: scopeId,
		KPIs: []DashboardKpi{
			{
				Key:        "operators",
				Label:      "Operators",
				Value:      0,
				Delta:      0,
				DeltaLabel: "Active Operators",
				Severity:   "info",
			},
			{
				Key:        "entities",
				Label:      "Entities",
				Value:      entityCount,
				Delta:      0,
				DeltaLabel: "OWPROV Scoped Entities",
				Severity:   "info",
			},
			{
				Key:        "venues",
				Label:      "Venues",
				Value:      venueCount,
				Delta:      0,
				DeltaLabel: "OWPROV Scoped Venues",
				Severity:   "info",
			},
			{
				Key:        "devices",
				Label:      "Devices",
				Value:      deviceCount,
				Delta:      0,
				DeltaLabel: fmt.Sprintf("%d Inventory APs", deviceCount),
				Severity:   "success",
			},
			{
				Key:        "users",
				Label:      "Users",
				Value:      0,
				Delta:      0,
				DeltaLabel: "Users",
				Severity:   "info",
			},
			{
				Key:        "alerts",
				Label:      "Alerts",
				Value:      0,
				Delta:      0,
				DeltaLabel: "0 critical",
				Severity:   "success",
			},
		},
		Health: HealthSummary{
			TotalDevices: deviceCount,
			Online:       deviceCount,
			Warning:      0,
			Offline:      0,
			Unknown:      0,
		},
		RecentAlerts: []RecentAlert{},
		Telemetry: TelemetrySummary{
			AirtimeUtilization: 0,
			NoiseFloorDbm:      0,
			CpuLoadPercent:     0,
			MemoryLoadPercent:  0,
			RssiDistribution:   []map[string]interface{}{},
		},
		SecurityProfiles: []TenantSecurityProfile{},
		TopVenues:        topVenuesList,
	}

	return res, nil
}
