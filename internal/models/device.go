package models

import "strings"

// NormalizeSerial normalizes a device serial number by trimming whitespace,
// converting to lower case, and stripping colon/hyphen separators.
func NormalizeSerial(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	return strings.TrimSpace(s)
}

// DeviceRebootRequest represents the payload for POST /api/v1/devices/:serial/reboot.
type DeviceRebootRequest struct {
	SerialNumber string `json:"serialNumber"`
	When         *int64 `json:"when,omitempty"`
}

// ExtendedEntity represents the nested entity metadata in an inventory tag.
type ExtendedEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ExtendedVenue represents the nested venue metadata in an inventory tag.
type ExtendedVenue struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ExtendedSubscriber represents the nested subscriber metadata in an inventory tag.
type ExtendedSubscriber struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	Description string `json:"description,omitempty"`
}

// ExtendedDeviceConfiguration represents the nested config metadata in an inventory tag.
type ExtendedDeviceConfiguration struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// InventoryExtendedInfo holds the extended metadata returned when withExtendedInfo=true.
type InventoryExtendedInfo struct {
	Entity              *ExtendedEntity              `json:"entity,omitempty"`
	Venue               *ExtendedVenue               `json:"venue,omitempty"`
	Subscriber          *ExtendedSubscriber          `json:"subscriber,omitempty"`
	DeviceConfiguration *ExtendedDeviceConfiguration `json:"deviceConfiguration,omitempty"`
}

// InventoryTag represents a device inventory tag in OWPROV.
type InventoryTag struct {
	SerialNumber        string                 `json:"serialNumber"`
	Name                string                 `json:"name,omitempty"`
	Description         string                 `json:"description,omitempty"`
	DeviceType          string                 `json:"deviceType,omitempty"`
	DevClass            string                 `json:"devClass,omitempty"`
	Entity              string                 `json:"entity,omitempty"`
	Venue               string                 `json:"venue,omitempty"`
	Subscriber          string                 `json:"subscriber,omitempty"`
	DeviceConfiguration string                 `json:"deviceConfiguration,omitempty"`
	RealMacAddress      string                 `json:"realMacAddress,omitempty"`
	ExtendedInfo        *InventoryExtendedInfo `json:"extendedInfo,omitempty"`
}

// InventoryTagListResponse represents the list envelope from OWPROV (supports both taglist and tagList).
type InventoryTagListResponse struct {
	TagList  []InventoryTag `json:"taglist"`
	TagList2 []InventoryTag `json:"tagList"`
}

// Tags returns whichever non-empty tag slice is populated.
func (r *InventoryTagListResponse) Tags() []InventoryTag {
	if len(r.TagList) > 0 {
		return r.TagList
	}
	return r.TagList2
}

// CountOnlyResponse represents the count envelope from OWPROV when countOnly=true.
type CountOnlyResponse struct {
	Count int `json:"count"`
}

// GatewayDeviceStatus represents real-time telemetry and status from OWGW.
type GatewayDeviceStatus struct {
	SerialNumber          string   `json:"serialNumber"`
	Compatible            string   `json:"compatible,omitempty"`
	DeviceType            string   `json:"deviceType,omitempty"`
	Connected             bool     `json:"connected"`
	Sanity                *float64 `json:"sanity,omitempty"`
	IPAddress             string   `json:"ipAddress,omitempty"`
	MacAddress            string   `json:"macAddress,omitempty"`
	MemoryUsed            *float64 `json:"memoryUsed,omitempty"`
	Load                  *float64 `json:"load,omitempty"`
	Temperature           *float64 `json:"temperature,omitempty"`
	Firmware              string   `json:"firmware,omitempty"`
	Started               *int64   `json:"started,omitempty"`
	Locale                string   `json:"locale,omitempty"`
	LastContact           any      `json:"lastContact,omitempty"`
	LastRecordedContact   *int64   `json:"lastRecordedContact,omitempty"`
	LastFWUpdate          *int64   `json:"lastFWUpdate,omitempty"`
	ConnectReason         string   `json:"connectReason,omitempty"`
	CertificateExpiryDate *int64   `json:"certificateExpiryDate,omitempty"`
	HasRADIUSSessions     any      `json:"hasRADIUSSessions,omitempty"`
	HasGPS                bool     `json:"hasGPS"`
	BlackListed           bool     `json:"blackListed"`
	Simulated             bool     `json:"simulated"`
	RestrictedDevice      bool     `json:"restrictedDevice"`
	Associations2G        int      `json:"associations_2G"`
	Associations5G        int      `json:"associations_5G"`
	Associations6G        int      `json:"associations_6G"`
	RxBytes               uint64   `json:"rxBytes"`
	TxBytes               uint64   `json:"txBytes"`
}

// GatewayDevicesWithStatusResponse represents the envelope returned by OWGW for ?deviceWithStatus=true.
type GatewayDevicesWithStatusResponse struct {
	DevicesWithStatus []GatewayDeviceStatus `json:"devicesWithStatus"`
}

// DeviceFleetItem is the unified device model combining inventory metadata with real-time gateway status.
type DeviceFleetItem struct {
	SerialNumber          string   `json:"serialNumber"`
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	Model                 string   `json:"model"`
	DeviceGroup           string   `json:"deviceGroup"`
	DeviceType            string   `json:"deviceType"`
	Connected             bool     `json:"connected"`
	Sanity                *float64 `json:"sanity,omitempty"`
	IPAddress             string   `json:"ipAddress"`
	MacAddress            string   `json:"macAddress,omitempty"`
	Locale                string   `json:"locale,omitempty"`
	MemoryUsedPct         *float64 `json:"memoryUsedPct,omitempty"`
	Load                  *float64 `json:"load,omitempty"`
	Temperature           *float64 `json:"temperature,omitempty"`
	Firmware              string   `json:"firmware,omitempty"`
	UptimeSeconds         *int64   `json:"uptimeSeconds,omitempty"`
	Started               *int64   `json:"started,omitempty"`
	LastContact           any      `json:"lastContact,omitempty"`
	LastContactTimestamp  *int64   `json:"lastContactTimestamp,omitempty"`
	LastContactText       string   `json:"lastContactText,omitempty"`
	LastRecordedContact   *int64   `json:"lastRecordedContact,omitempty"`
	LastFWUpdate          *int64   `json:"lastFWUpdate,omitempty"`
	ConnectReason         string   `json:"connectReason,omitempty"`
	CertificateExpiryDate *int64   `json:"certificateExpiryDate,omitempty"`
	HasRADIUSSessions     bool     `json:"hasRADIUSSessions"`
	HasGPS                bool     `json:"hasGPS"`
	Blacklisted           bool     `json:"blacklisted"`
	Simulated             bool     `json:"simulated"`
	Restricted            bool     `json:"restricted"`
	AssociationsTotal     int      `json:"associationsTotal"`
	Associations2G        int      `json:"associations_2G"`
	Associations5G        int      `json:"associations_5G"`
	Associations6G        int      `json:"associations_6G"`
	RxBytes               uint64   `json:"rxBytes"`
	TxBytes               uint64   `json:"txBytes"`
	EntityID              string   `json:"entityId,omitempty"`
	EntityName            string   `json:"entityName,omitempty"`
	VenueID               string   `json:"venueId,omitempty"`
	VenueName             string   `json:"venueName,omitempty"`
	SubscriberID          string   `json:"subscriberId,omitempty"`
	SubscriberEmail       string   `json:"subscriberEmail,omitempty"`
	ConfigID              string   `json:"configId,omitempty"`
	ConfigName            string   `json:"configName,omitempty"`
}

// PaginatedDevicesResponse is the response structure for GET /api/v1/devices.
type PaginatedDevicesResponse struct {
	Devices    []DeviceFleetItem `json:"devices"`
	TotalCount int               `json:"totalCount"`
	PageIndex  int               `json:"pageIndex"`
	PageSize   int               `json:"pageSize"`
	TotalPages int               `json:"totalPages"`
}

// InventorySerialsResponse is the response structure returned by OWPROV when serialOnly=true.
type InventorySerialsResponse struct {
	SerialNumbers []string `json:"serialNumbers"`
}

// BlacklistDevice represents a blacklisted device entry.
type BlacklistDevice struct {
	SerialNumber string `json:"serialNumber"`
	Reason       string `json:"reason"`
	Author       string `json:"author,omitempty"`
	Created      uint64 `json:"created,omitempty"`
}

// BlacklistListResponse represents the list envelope from OWGW or MDU for blacklist.
type BlacklistListResponse struct {
	Devices []BlacklistDevice `json:"devices"`
}

// AddBlacklistRequest represents payload to add a device to blacklist.
type AddBlacklistRequest struct {
	SerialNumber string `json:"serialNumber,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// DeviceNotesUpdateRequest is the only update supported by MDU's device PUT endpoint.
type DeviceNotesUpdateRequest struct {
	SerialNumber string            `json:"serialNumber,omitempty"`
	Notes        []DeviceNoteInput `json:"notes"`
}

type DeviceNoteInput struct {
	Note string `json:"note"`
	// Accepted for compatibility with the UI; OWGW assigns the actual timestamp.
	Created int64 `json:"created,omitempty"`
}

// DirectDeviceConfigurationRequest contains a concrete uCentral configuration
// supplied by an operator. MDU creates the OWGW command envelope itself.
type DirectDeviceConfigurationRequest struct {
	Configuration map[string]any `json:"configuration"`
	When          uint64         `json:"when,omitempty"`
}

// DeviceCommandAuthRequest represents the request payload to verify authorization for a device command.
type DeviceCommandAuthRequest struct {
	SerialNumber string `json:"serialNumber"`
	Action       string `json:"action"`
}

// DeviceCommandAuthResult represents the response from OWPROV device command authorization check.
type DeviceCommandAuthResult struct {
	Allowed      bool   `json:"allowed"`
	Resource     string `json:"resource"`
	Action       string `json:"action"`
	SerialNumber string `json:"serialNumber"`
	UserID       string `json:"userId"`
	PolicyID     string `json:"policyId,omitempty"`
	PolicyName   string `json:"policyName,omitempty"`
	Reason       string `json:"reason"`
	Details      string `json:"details"`
}
