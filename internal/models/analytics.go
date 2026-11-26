package models

// AnalyticsBoard represents a monitoring board in OWANALYTICS.
type AnalyticsBoard struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	VenueList   []AnalyticsBoardVenue `json:"venueList"`
}

// AnalyticsBoardVenue describes a venue being monitored within a board.
type AnalyticsBoardVenue struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Interval         int    `json:"interval"`
	Retention        int    `json:"retention"`
	MonitorSubVenues bool   `json:"monitorSubVenues"`
}

// CreateAnalyticsBoardRequest represents the payload to create a monitoring board in OWANALYTICS.
type CreateAnalyticsBoardRequest struct {
	Name      string                `json:"name"`
	VenueList []AnalyticsBoardVenue `json:"venueList"`
}

// AnalyticsBoardDevice represents device status telemetry returned by OWANALYTICS.
type AnalyticsBoardDevice struct {
	SerialNumber   string  `json:"serialNumber"`
	Connected      bool    `json:"connected"`
	DeviceType     string  `json:"deviceType"`
	Health         float64 `json:"health"`
	Memory         float64 `json:"memory"`
	Uptime         int64   `json:"uptime"`
	LastContact    int64   `json:"lastContact"`
	Associations2G int     `json:"associations_2g"`
	Associations5G int     `json:"associations_5g"`
	Associations6G int     `json:"associations_6g"`
	LastFirmware   string  `json:"lastFirmware,omitempty"`
	ConnectionIP   string  `json:"connectionIp,omitempty"`
}

// AnalyticsBoardDevicesResponse represents the response containing device telemetry from OWANALYTICS.
type AnalyticsBoardDevicesResponse struct {
	Devices []AnalyticsBoardDevice `json:"devices"`
}

// VenueMonitoringSummary aggregates high-level KPIs for a venue.
type VenueMonitoringSummary struct {
	ConnectedDevices    int     `json:"connectedDevices"`
	DisconnectedDevices int     `json:"disconnectedDevices"`
	TotalDevices        int     `json:"totalDevices"`
	AvgHealth           float64 `json:"avgHealth"`
	AvgMemoryUsed       float64 `json:"avgMemoryUsed"`
	TwoGAssociations    int     `json:"twoGAssociations"`
	FiveGAssociations   int     `json:"fiveGAssociations"`
	SixGAssociations    int     `json:"sixGAssociations"`
	TotalAssociations   int     `json:"totalAssociations"`
}

// VenueMonitoringResponse is returned by MDU for venue monitoring status.
type VenueMonitoringResponse struct {
	Active    bool                    `json:"active"`
	VenueID   string                  `json:"venueId"`
	BoardID   string                  `json:"boardId,omitempty"`
	Interval  int                     `json:"interval,omitempty"`
	Retention int                     `json:"retention,omitempty"`
	Summary   *VenueMonitoringSummary `json:"summary,omitempty"`
	Devices   []AnalyticsBoardDevice  `json:"devices,omitempty"`
}

// StartVenueMonitoringRequest is the payload to activate monitoring on a venue.
type StartVenueMonitoringRequest struct {
	Name             string `json:"name,omitempty"`
	Interval         int    `json:"interval,omitempty"`
	Retention        int    `json:"retention,omitempty"`
	MonitorSubVenues bool   `json:"monitorSubVenues,omitempty"`
}
