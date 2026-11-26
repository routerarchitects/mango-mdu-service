package models

// OWProvTreeNode mirrors the authorized hierarchy returned by
// GET /api/v1/entity?getTree=true. Entity children and venue children are kept
// separate in OWPROV and normalized by the property service for the UI.
type OWProvTreeNode struct {
	Type     string           `json:"type"`
	Name     string           `json:"name"`
	UUID     string           `json:"uuid"`
	Children []OWProvTreeNode `json:"children,omitempty"`
	Venues   []OWProvTreeNode `json:"venues,omitempty"`
	Devices  []string         `json:"devices,omitempty"`
}

// PropertyTreeNode is the normalized hierarchy model. Every OWPROV entity is
// represented as a property and every OWPROV venue is represented as a venue.
// Device identifiers from OWPROV's structural tree are discarded. Operational
// views populate Devices from separately authorized inventory reads, and the
// field must never be exposed by structural tree or portfolio responses.
type PropertyTreeNode struct {
	ID          string             `json:"id"`
	Type        string             `json:"type"`
	Name        string             `json:"name"`
	Devices     []string           `json:"-"`
	Children    []PropertyTreeNode `json:"children"`
	Summary     *ScopeSummary      `json:"summary,omitempty"`
	DeviceNames map[string]string  `json:"-"`
}

type PropertyTreeResponse struct {
	Nodes []PropertyTreeNode `json:"nodes"`
}

// PropertyMapDevice is the minimal Device projection needed by the on-demand
// hierarchy map. Availability is factual and never implies Device health.
type PropertyMapDevice struct {
	SerialNumber string   `json:"serialNumber"`
	Name         string   `json:"name,omitempty"`
	Availability string   `json:"availability"`
	Sanity       *float64 `json:"sanity,omitempty"`
}

// PropertyMapNode preserves direct scope ownership so the UI can render each
// Device below the Entity or Venue where OWPROV assigned it.
type PropertyMapNode struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Name     string              `json:"name"`
	Devices  []PropertyMapDevice `json:"devices"`
	Children []PropertyMapNode   `json:"children"`
}

type PropertyMapResponse struct {
	Nodes       []PropertyMapNode `json:"nodes"`
	DataQuality DataQuality       `json:"dataQuality"`
}

type PropertyPortfolioResponse struct {
	Nodes            []PropertyTreeNode `json:"nodes"`
	Summary          ScopeSummary       `json:"summary"`
	DataQuality      DataQuality        `json:"dataQuality"`
	PropertyCount    int                `json:"propertyCount"`
	VenueCount       int                `json:"venueCount"`
	NestedVenueCount int                `json:"nestedVenueCount"`
}

type DataQualityError struct {
	Source    string `json:"source"`
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
}

type DataQuality struct {
	State              string             `json:"state"`
	Partial            bool               `json:"partial"`
	UnknownDeviceCount int                `json:"unknownDeviceCount"`
	AsOf               string             `json:"asOf"`
	Errors             []DataQualityError `json:"errors"`
}

type ScopeSummary struct {
	TotalDevices      int `json:"totalDevices"`
	OnlineDevices     int `json:"onlineDevices"`
	OfflineDevices    int `json:"offlineDevices"`
	UnknownDevices    int `json:"unknownDevices"`
	StatusMeasuredFor int `json:"statusMeasuredFor"`
}

type ChildScopeSummary struct {
	ID      string       `json:"id"`
	Type    string       `json:"type"`
	Name    string       `json:"name"`
	Summary ScopeSummary `json:"summary"`
}

type ScopeOverviewResponse struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Name         string              `json:"name"`
	Summary      ScopeSummary        `json:"summary"`
	Children     []ChildScopeSummary `json:"children"`
	DataQuality  DataQuality         `json:"dataQuality"`
	IncludesDesc bool                `json:"includesDescendants"`
}

type PropertyDeviceItem struct {
	SerialNumber string   `json:"serialNumber"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	DeviceType   string   `json:"deviceType,omitempty"`
	DeviceGroup  string   `json:"deviceGroup,omitempty"`
	EntityID     string   `json:"entityId,omitempty"`
	EntityName   string   `json:"entityName,omitempty"`
	VenueID      string   `json:"venueId,omitempty"`
	VenueName    string   `json:"venueName,omitempty"`
	Availability string   `json:"availability"`
	Connected    bool     `json:"connected"`
	StatusKnown  bool     `json:"statusKnown"`
	IPAddress    string   `json:"ipAddress,omitempty"`
	Firmware     string   `json:"firmware,omitempty"`
	LastContact  any      `json:"lastContact,omitempty"`
	Sanity       *float64 `json:"sanity,omitempty"`
}

type ScopedDevicesResponse struct {
	Devices     []PropertyDeviceItem `json:"devices"`
	TotalCount  int                  `json:"totalCount"`
	PageIndex   int                  `json:"pageIndex"`
	PageSize    int                  `json:"pageSize"`
	TotalPages  int                  `json:"totalPages"`
	Summary     ScopeSummary         `json:"summary"`
	DataQuality DataQuality          `json:"dataQuality"`
}

type ScopedDevicesQuery struct {
	PageIndex    int
	PageSize     int
	Search       string
	VenueID      string
	Availability string
	DeviceType   string
	Attention    string
}
