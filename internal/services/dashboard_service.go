package services

import (
	"context"
	"fmt"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/analytics"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/gw"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
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
	provClient      *prov.Client
	secClient       *sec.Client
	gwClient        *gw.Client
	analyticsClient *analytics.Client
}

func NewDashboardService(
	provClient *prov.Client,
	secClient *sec.Client,
	gwClient *gw.Client,
	analyticsClient *analytics.Client,
) *DashboardService {
	return &DashboardService{
		provClient:      provClient,
		secClient:       secClient,
		gwClient:        gwClient,
		analyticsClient: analyticsClient,
	}
}

func (s *DashboardService) GetDashboard(ctx context.Context, reqCtx prov.RequestContext, scopeId string) (*DashboardResponse, error) {
	if s.provClient == nil {
		return nil, fmt.Errorf("owprov service discovery client is uninitialized")
	}

	// 1. Fetch real entities from OWPROV (fail fast on downstream error)
	entities, err := s.provClient.ListEntities(reqCtx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("owprov ListEntities error: %w", err)
	}

	// 2. Fetch real venues from OWPROV (fail fast on downstream error)
	venues, err := s.provClient.ListVenues(reqCtx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("owprov ListVenues error: %w", err)
	}

	entityCount := len(entities)
	venueCount := len(venues)

	// 3. Fetch real inventory devices from OWPROV
	var deviceCount int
	inventoryTags, err := s.provClient.ListInventory(reqCtx, 1000, 0)
	if err == nil {
		deviceCount = len(inventoryTags)
	} else {
		for _, v := range venues {
			deviceCount += len(v.Devices)
		}
	}

	// 4. Fetch real operators from OWPROV
	var operatorCount int
	operators, err := s.provClient.ListOperators(reqCtx)
	if err == nil {
		operatorCount = len(operators)
	}

	// 5. Fetch real users from OWSEC (or fallback to OWPROV management roles)
	var userCount int
	if s.secClient != nil {
		secUsers, err := s.secClient.ListUsers(ctx, reqCtx.BearerToken)
		if err == nil && len(secUsers) > 0 {
			userCount = len(secUsers)
		}
	}
	if userCount == 0 {
		roles, err := s.provClient.ListManagementRoles(reqCtx, 1000, 0)
		if err == nil {
			userCount = len(roles)
		}
	}

	// 6. Fetch live device connectivity status from OWGW
	var onlineDevices int = 0
	var offlineDevices int = deviceCount
	if s.gwClient != nil {
		gwDevices, err := s.gwClient.ListDevicesStatus(reqCtx)
		if err == nil && len(gwDevices) > 0 {
			onlineCount := 0
			for _, d := range gwDevices {
				if d.Connected {
					onlineCount++
				}
			}
			onlineDevices = onlineCount
			if deviceCount >= onlineDevices {
				offlineDevices = deviceCount - onlineDevices
			}
		}
	}

	// 7. Fetch live telemetry metrics from OWANALYTICS (strictly 0 / empty if no live data)
	telemetry := TelemetrySummary{
		AirtimeUtilization: 0,
		NoiseFloorDbm:      0,
		CpuLoadPercent:     0,
		MemoryLoadPercent:  0,
		RssiDistribution:   []map[string]interface{}{},
	}
	if s.analyticsClient != nil {
		anData, err := s.analyticsClient.GetTelemetry(reqCtx)
		if err == nil && anData != nil {
			telemetry.AirtimeUtilization = anData.AirtimeUtilization
			telemetry.NoiseFloorDbm = anData.NoiseFloorDbm
			telemetry.CpuLoadPercent = anData.CpuLoadPercent
			telemetry.MemoryLoadPercent = anData.MemoryLoadPercent
			if len(anData.RssiDistribution) > 0 {
				telemetry.RssiDistribution = anData.RssiDistribution
			}
		}
	}

	// 8. Fetch real security configuration profiles from OWPROV
	securityProfiles := make([]TenantSecurityProfile, 0)
	configs, err := s.provClient.ListConfigurations(reqCtx, 1000, 0)
	if err == nil && len(configs) > 0 {
		totalCfg := len(configs)
		for _, cfg := range configs {
			pct := 100 / totalCfg
			securityProfiles = append(securityProfiles, TenantSecurityProfile{
				Name:       cfg.Name,
				Percentage: pct,
				Count:      1,
				Color:      "bg-emerald-500",
			})
		}
	}

	// 9. Format top venues directly from OWPROV real venue data
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
				Value:      operatorCount,
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
				DeltaLabel: fmt.Sprintf("%d Online APs", onlineDevices),
				Severity:   "success",
			},
			{
				Key:        "users",
				Label:      "Users",
				Value:      userCount,
				Delta:      0,
				DeltaLabel: "Registered Users",
				Severity:   "info",
			},
			{
				Key:        "alerts",
				Label:      "Alerts",
				Value:      offlineDevices,
				Delta:      0,
				DeltaLabel: fmt.Sprintf("%d critical", offlineDevices),
				Severity:   "success",
			},
		},
		Health: HealthSummary{
			TotalDevices: deviceCount,
			Online:       onlineDevices,
			Warning:      0,
			Offline:      offlineDevices,
			Unknown:      0,
		},
		RecentAlerts:     []RecentAlert{},
		Telemetry:        telemetry,
		SecurityProfiles: securityProfiles,
		TopVenues:        topVenuesList,
	}

	return res, nil
}
