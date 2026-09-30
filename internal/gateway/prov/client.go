package prov

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type RequestContext struct {
	Context       context.Context
	BearerToken   string
	RequestID     string
	CorrelationID string
}

type Client struct {
	discovery    *servicediscovery.Discovery
	httpClient   *http.Client
	internalName string
	BaseURL      string // For testing override
}

func NewClient(discovery *servicediscovery.Discovery, tlsRootCA string, internalName string) (*Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	if strings.TrimSpace(tlsRootCA) != "" {
		pemBytes, err := os.ReadFile(tlsRootCA)
		if err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(pemBytes) {
				tlsConfig.RootCAs = pool
			}
		}
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &Client{
		discovery: discovery,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		internalName: internalName,
	}, nil
}

func (c *Client) sendRequest(reqCtx RequestContext, method, path string, body io.Reader) (*http.Response, error) {
	var urlStr string
	var apiKey string

	if c.BaseURL != "" {
		urlStr = strings.TrimSuffix(c.BaseURL, "/") + path
		apiKey = "mock-api-key"
	} else {
		if c.discovery != nil && c.discovery.Store() != nil {
			if inst := c.discovery.Store().GetServiceInstances("owprov"); inst != nil {
				pubURL, errPub := url.Parse(inst.PublicEndPoint)
				privURL, errPriv := url.Parse(inst.PrivateEndPoint)
				if errPub == nil && errPriv == nil && privURL.Hostname() != "" && pubURL.Port() != "" {
					urlStr = fmt.Sprintf("%s://%s:%s%s", pubURL.Scheme, privURL.Hostname(), pubURL.Port(), path)
				} else if errPriv == nil && privURL.Hostname() != "" {
					urlStr = fmt.Sprintf("https://%s:16005%s", privURL.Hostname(), path)
				} else {
					urlStr = strings.TrimSuffix(inst.PublicEndPoint, "/") + path
				}
				apiKey = inst.Key
			}
		}
		if urlStr == "" {
			urlStr = "https://openwifi-owprov-1:16005" + path
		}
	}

	req, err := http.NewRequestWithContext(reqCtx.Context, method, urlStr, body)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create http request", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if reqCtx.BearerToken != "" {
		if !strings.HasPrefix(strings.ToLower(reqCtx.BearerToken), "bearer ") {
			req.Header.Set("Authorization", "Bearer "+reqCtx.BearerToken)
		} else {
			req.Header.Set("Authorization", reqCtx.BearerToken)
		}
	} else {
		req.Header.Set("X-API-KEY", apiKey)
		req.Header.Set("X-INTERNAL-NAME", c.internalName)
	}

	if reqCtx.RequestID != "" {
		req.Header.Set("X-Request-Id", reqCtx.RequestID)
	}
	if reqCtx.CorrelationID != "" {
		req.Header.Set("X-Correlation-Id", reqCtx.CorrelationID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("DEBUG prov sendRequest FAIL", "method", method, "url", urlStr, "apiKey", apiKey, "error", err)
		return nil, apperror.Wrap(apperror.Code("DOWNSTREAM_UNAVAILABLE"), "owprov request failed", err)
	}
	slog.Error("DEBUG prov sendRequest SUCCESS", "method", method, "url", urlStr, "apiKey", apiKey, "status", resp.StatusCode)

	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, apperror.New(apperror.Code("DOWNSTREAM_UNAVAILABLE"), fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return resp, nil
}

// Downstream models
type ProvObjectInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Created     int64  `json:"created"`
	Modified    int64  `json:"modified"`
}

type ProvOperator struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Created        int64  `json:"created"`
	Modified       int64  `json:"modified"`
	RegistrationID string `json:"registrationId"`
	EntityID       string `json:"entityId"`
}

// MarshalJSON implements custom marshaling to omit entityId for serialization to owprov
func (o ProvOperator) MarshalJSON() ([]byte, error) {
	type Alias ProvOperator
	return json.Marshal(&struct {
		Alias
		EntityID string `json:"entityId,omitempty"`
	}{
		Alias:    Alias(o),
		EntityID: "",
	})
}

type ProvEntity struct {
	Info             ProvObjectInfo `json:"info"` // mapped in allOf downstream
	Parent           string         `json:"parent,omitempty"`
	OperatorID       string         `json:"operatorId,omitempty"`
	Children         []string       `json:"children,omitempty"`
	Venues           []string       `json:"venues,omitempty"`
	Contacts         []string       `json:"contacts,omitempty"`
	Locations        []string       `json:"locations,omitempty"`
	ManagementPolicy string         `json:"managementPolicy,omitempty"`
	Devices          []string       `json:"devices,omitempty"`
	ManagementRoles  []string       `json:"managementRoles,omitempty"`
	Type             string         `json:"type,omitempty"` // normal, subscriber
}

// UnmarshalJSON implements custom unmarshaling to extract allOf ObjectInfo fields
func (e *ProvEntity) UnmarshalJSON(data []byte) error {
	type Alias ProvEntity
	var aux struct {
		*Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}
	aux.Alias = (*Alias)(e)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	e.Info = ProvObjectInfo{
		ID:          aux.ID,
		Name:        aux.Name,
		Description: aux.Description,
		Created:     aux.Created,
		Modified:    aux.Modified,
	}
	return nil
}

// MarshalJSON implements custom marshaling to flatten allOf ObjectInfo fields
func (e ProvEntity) MarshalJSON() ([]byte, error) {
	type Alias ProvEntity
	return json.Marshal(&struct {
		Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}{
		Alias:       Alias(e),
		ID:          e.Info.ID,
		Name:        e.Info.Name,
		Description: e.Info.Description,
		Created:     e.Info.Created,
		Modified:    e.Info.Modified,
	})
}

type ProvVenue struct {
	Info             ProvObjectInfo `json:"info"` // mapped in allOf downstream
	Entity           string         `json:"entity,omitempty"`
	Parent           string         `json:"parent,omitempty"`
	Children         []string       `json:"children,omitempty"`
	ManagementPolicy string         `json:"managementPolicy,omitempty"`
	Devices          []string       `json:"devices,omitempty"`
	Contacts         []string       `json:"contacts,omitempty"`
	Location         string         `json:"location,omitempty"`
	ManagementRoles  []string       `json:"managementRoles,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling to extract allOf ObjectInfo fields
func (v *ProvVenue) UnmarshalJSON(data []byte) error {
	type Alias ProvVenue
	var aux struct {
		*Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}
	aux.Alias = (*Alias)(v)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	v.Info = ProvObjectInfo{
		ID:          aux.ID,
		Name:        aux.Name,
		Description: aux.Description,
		Created:     aux.Created,
		Modified:    aux.Modified,
	}
	return nil
}

// MarshalJSON implements custom marshaling to flatten allOf ObjectInfo fields
func (v ProvVenue) MarshalJSON() ([]byte, error) {
	type Alias ProvVenue
	return json.Marshal(&struct {
		Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}{
		Alias:       Alias(v),
		ID:          v.Info.ID,
		Name:        v.Info.Name,
		Description: v.Info.Description,
		Created:     v.Info.Created,
		Modified:    v.Info.Modified,
	})
}

type ProvManagementPolicyEntry struct {
	Users     []string `json:"users,omitempty"`
	Resources []string `json:"resources"`
	Access    []string `json:"access"` // READ, MODIFY, DELETE, NOACCESS
	Policy    string   `json:"policy,omitempty"`
}

type ProvManagementPolicy struct {
	Info    ProvObjectInfo              `json:"info"` // mapped in allOf downstream
	Entries []ProvManagementPolicyEntry `json:"entries"`
	Entity  string                      `json:"entity,omitempty"`
	Venue   string                      `json:"venue,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling to extract allOf ObjectInfo fields
func (p *ProvManagementPolicy) UnmarshalJSON(data []byte) error {
	type Alias ProvManagementPolicy
	var aux struct {
		*Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}
	aux.Alias = (*Alias)(p)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	p.Info = ProvObjectInfo{
		ID:          aux.ID,
		Name:        aux.Name,
		Description: aux.Description,
		Created:     aux.Created,
		Modified:    aux.Modified,
	}
	return nil
}

// MarshalJSON implements custom marshaling to flatten allOf ObjectInfo fields
func (p ProvManagementPolicy) MarshalJSON() ([]byte, error) {
	type Alias ProvManagementPolicy
	return json.Marshal(&struct {
		Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}{
		Alias:       Alias(p),
		ID:          p.Info.ID,
		Name:        p.Info.Name,
		Description: p.Info.Description,
		Created:     p.Info.Created,
		Modified:    p.Info.Modified,
	})
}

type ProvManagementRole struct {
	Info             ProvObjectInfo `json:"info"` // mapped in allOf downstream
	ManagementPolicy string         `json:"managementPolicy"`
	Users            []string       `json:"users"`
	Entity           string         `json:"entity,omitempty"`
	Venue            string         `json:"venue,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling to extract allOf ObjectInfo fields
func (r *ProvManagementRole) UnmarshalJSON(data []byte) error {
	type Alias ProvManagementRole
	var aux struct {
		*Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}
	aux.Alias = (*Alias)(r)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.Info = ProvObjectInfo{
		ID:          aux.ID,
		Name:        aux.Name,
		Description: aux.Description,
		Created:     aux.Created,
		Modified:    aux.Modified,
	}
	return nil
}

// MarshalJSON implements custom marshaling to flatten allOf ObjectInfo fields
func (r ProvManagementRole) MarshalJSON() ([]byte, error) {
	type Alias ProvManagementRole
	return json.Marshal(&struct {
		Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}{
		Alias:       Alias(r),
		ID:          r.Info.ID,
		Name:        r.Info.Name,
		Description: r.Info.Description,
		Created:     r.Info.Created,
		Modified:    r.Info.Modified,
	})
}

type ProvSignupEntry struct {
	Info           ProvObjectInfo `json:"info"` // mapped in allOf downstream
	Email          string         `json:"email"`
	UserID         string         `json:"userId"`
	OperatorID     string         `json:"operatorId"`
	MacAddress     string         `json:"macAddress"`
	SerialNumber   string         `json:"serialNumber"`
	Status         string         `json:"status"`
	RegistrationID string         `json:"registrationId"`
}

// UnmarshalJSON implements custom unmarshaling to extract allOf ObjectInfo fields
func (s *ProvSignupEntry) UnmarshalJSON(data []byte) error {
	type Alias ProvSignupEntry
	var aux struct {
		*Alias
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     int64  `json:"created"`
		Modified    int64  `json:"modified"`
	}
	aux.Alias = (*Alias)(s)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	s.Info = ProvObjectInfo{
		ID:          aux.ID,
		Name:        aux.Name,
		Description: aux.Description,
		Created:     aux.Created,
		Modified:    aux.Modified,
	}
	return nil
}

// Lists/Responses wrappers
type ProvEntityList struct {
	Entities []ProvEntity `json:"entities"`
}

type ProvVenueList struct {
	Venues []ProvVenue `json:"venues"`
}

type ProvManagementPolicyList struct {
	Policies []ProvManagementPolicy `json:"policies"`
}

type ProvManagementRoleList struct {
	Roles []ProvManagementRole `json:"roles"`
}

type ProvSignupList struct {
	Signups []ProvSignupEntry `json:"signups"`
}

type ProvTreeNode struct {
	Type     string         `json:"type"` // entity, venue
	Name     string         `json:"name"`
	UUID     string         `json:"uuid"`
	ID       string         `json:"id"`
	Children []ProvTreeNode `json:"children,omitempty"`
	Venues   []ProvTreeNode `json:"venues,omitempty"`
}

func (n *ProvTreeNode) GetID() string {
	if n.ID != "" {
		return n.ID
	}
	return n.UUID
}

// Operator APIs----------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func (c *Client) GetOperator(reqCtx RequestContext, operatorID string) (*ProvOperator, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/operator/"+operatorID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "operator not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var op ProvOperator
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode operator details", err)
	}

	return &op, nil
}

func (c *Client) UpdateOperator(reqCtx RequestContext, operatorID string, op *ProvOperator) (*ProvOperator, error) {
	bodyBytes, err := json.Marshal(op)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal operator body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPut, "/api/v1/operator/"+operatorID, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "operator not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var updated ProvOperator
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode operator details", err)
	}

	return &updated, nil
}

func (c *Client) DeleteOperator(reqCtx RequestContext, operatorID string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/operator/"+operatorID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return apperror.New(apperror.CodeNotFound, "operator not found")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

// Tree APIs----------------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func (c *Client) GetTree(reqCtx RequestContext) (*ProvTreeNode, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/entity?getTree=true", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov tree returned status %d: %s", resp.StatusCode, string(body)))
	}

	var tree ProvTreeNode
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode tree details", err)
	}

	return &tree, nil
}

type ProvExtendedVenueInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ProvExtendedInfo struct {
	Venue ProvExtendedVenueInfo `json:"venue"`
}

type ProvInventoryTag struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	SerialNumber string           `json:"serialNumber"`
	Venue        string           `json:"-"`
	Entity       string           `json:"entity"`
	DevClass     string           `json:"devClass"`
	DeviceType   string           `json:"deviceType"`
	Platform     string           `json:"platform"`
	ExtendedInfo ProvExtendedInfo `json:"extendedInfo"`
}

func (t *ProvInventoryTag) UnmarshalJSON(data []byte) error {
	type Alias ProvInventoryTag
	var aux struct {
		*Alias
		VenueRaw json.RawMessage `json:"venue"`
	}
	aux.Alias = (*Alias)(t)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.VenueRaw) > 0 {
		var s string
		if err := json.Unmarshal(aux.VenueRaw, &s); err == nil {
			t.Venue = s
		} else {
			var obj struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(aux.VenueRaw, &obj); err == nil {
				t.Venue = obj.ID
			}
		}
	}
	return nil
}

type ProvInventoryList struct {
	Taglist []ProvInventoryTag `json:"taglist"`
}

type ProvOperatorList struct {
	Operators []ProvOperator `json:"operators"`
}

func (c *Client) ListConfigurationsPaged(reqCtx RequestContext, limit, offset int) ([]ProvConfiguration, error) {
	u := fmt.Sprintf("/api/v1/configuration?limit=%d&offset=%d", limit, offset)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov configuration returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvConfigurationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode configuration list", err)
	}

	return list.Configurations, nil
}

// Entity APIs--------------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func (c *Client) ListEntities(reqCtx RequestContext, limit, offset int) ([]ProvEntity, error) {
	u := fmt.Sprintf("/api/v1/entity?limit=%d&offset=%d", limit, offset)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvEntityList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode entity list", err)
	}

	return list.Entities, nil
}

func (c *Client) ListInventory(reqCtx RequestContext, args ...int) ([]ProvInventoryTag, error) {
	limit := 1000
	offset := 0
	if len(args) > 0 && args[0] > 0 {
		limit = args[0]
	}
	if len(args) > 1 {
		offset = args[1]
	}
	u := fmt.Sprintf("/api/v1/inventory?withExtendedInfo=true&limit=%d&offset=%d", limit, offset)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov inventory returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvInventoryList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode inventory list", err)
	}

	return list.Taglist, nil
}

func (c *Client) ListOperators(reqCtx RequestContext) ([]ProvOperator, error) {
	u := "/api/v1/operator?withExtendedInfo=true"
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov operator returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvOperatorList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode operator list", err)
	}

	return list.Operators, nil
}

func (c *Client) ListManagementRoles(reqCtx RequestContext, limit, offset int) ([]ProvManagementRole, error) {
	u := fmt.Sprintf("/api/v1/managementRole?limit=%d&offset=%d", limit, offset)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov managementRole returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvManagementRoleList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode managementRole list", err)
	}

	return list.Roles, nil
}

func (c *Client) GetEntity(reqCtx RequestContext, uuid string) (*ProvEntity, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/entity/"+uuid, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "entity not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var entity ProvEntity
	if err := json.NewDecoder(resp.Body).Decode(&entity); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode entity details", err)
	}

	return &entity, nil
}

func (c *Client) CreateEntity(reqCtx RequestContext, uuid string, entity *ProvEntity) (*ProvEntity, error) {
	bodyBytes, err := json.Marshal(entity)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal entity body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, "/api/v1/entity/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusConflict {
			return nil, apperror.New(apperror.CodeConflict, "entity already exists")
		}
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var created ProvEntity
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode created entity", err)
	}

	return &created, nil
}

func (c *Client) UpdateEntity(reqCtx RequestContext, uuid string, entity *ProvEntity) (*ProvEntity, error) {
	bodyBytes, err := json.Marshal(entity)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal entity body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPut, "/api/v1/entity/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "entity not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var updated ProvEntity
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode updated entity", err)
	}

	return &updated, nil
}

func (c *Client) DeleteEntity(reqCtx RequestContext, uuid string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/entity/"+uuid, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return apperror.New(apperror.CodeNotFound, "entity not found")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

func (c *Client) ListVenues(reqCtx RequestContext, limit, offset int) ([]ProvVenue, error) {
	u := fmt.Sprintf("/api/v1/venue?limit=%d&offset=%d", limit, offset)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvVenueList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode venue list", err)
	}

	return list.Venues, nil
}

// Venue APIs-------------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func (c *Client) GetVenue(reqCtx RequestContext, uuid string) (*ProvVenue, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/venue/"+uuid, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "venue not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var venue ProvVenue
	if err := json.NewDecoder(resp.Body).Decode(&venue); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode venue details", err)
	}

	return &venue, nil
}

func (c *Client) CreateVenue(reqCtx RequestContext, uuid string, venue *ProvVenue) (*ProvVenue, error) {
	bodyBytes, err := json.Marshal(venue)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal venue body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, "/api/v1/venue/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusConflict {
			return nil, apperror.New(apperror.CodeConflict, "venue already exists")
		}
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var created ProvVenue
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode created venue", err)
	}

	return &created, nil
}

func (c *Client) UpdateVenue(reqCtx RequestContext, uuid string, venue *ProvVenue) (*ProvVenue, error) {
	bodyBytes, err := json.Marshal(venue)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal venue body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPut, "/api/v1/venue/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "venue not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var updated ProvVenue
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode updated venue", err)
	}

	return &updated, nil
}

func (c *Client) DeleteVenue(reqCtx RequestContext, uuid string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/venue/"+uuid, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return apperror.New(apperror.CodeNotFound, "venue not found")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

// Management Policy APIs----------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func (c *Client) ListPolicies(reqCtx RequestContext, limit, offset int) ([]ProvManagementPolicy, error) {
	u := fmt.Sprintf("/api/v1/managementPolicy?limit=%d&offset=%d", limit, offset)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvManagementPolicyList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode policy list", err)
	}

	return list.Policies, nil
}

func (c *Client) GetPolicy(reqCtx RequestContext, uuid string) (*ProvManagementPolicy, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/managementPolicy/"+uuid, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "policy not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var policy ProvManagementPolicy
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode policy details", err)
	}

	return &policy, nil
}

func (c *Client) CreatePolicy(reqCtx RequestContext, uuid string, policy *ProvManagementPolicy) (*ProvManagementPolicy, error) {
	bodyBytes, err := json.Marshal(policy)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal policy body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, "/api/v1/managementPolicy/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusConflict {
			return nil, apperror.New(apperror.CodeConflict, "policy already exists")
		}
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var created ProvManagementPolicy
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode created policy", err)
	}

	return &created, nil
}

func (c *Client) UpdatePolicy(reqCtx RequestContext, uuid string, policy *ProvManagementPolicy) (*ProvManagementPolicy, error) {
	bodyBytes, err := json.Marshal(policy)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal policy body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPut, "/api/v1/managementPolicy/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "policy not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var updated ProvManagementPolicy
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode updated policy", err)
	}

	return &updated, nil
}

func (c *Client) DeletePolicy(reqCtx RequestContext, uuid string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/managementPolicy/"+uuid, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return apperror.New(apperror.CodeNotFound, "policy not found")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

// Management Role APIs----------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func (c *Client) ListRoles(reqCtx RequestContext, limit, offset int) ([]ProvManagementRole, error) {
	u := fmt.Sprintf("/api/v1/managementRole?limit=%d&offset=%d", limit, offset)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvManagementRoleList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode role list", err)
	}

	return list.Roles, nil
}

func (c *Client) GetRole(reqCtx RequestContext, uuid string) (*ProvManagementRole, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/managementRole/"+uuid, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "role not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var role ProvManagementRole
	if err := json.NewDecoder(resp.Body).Decode(&role); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode role details", err)
	}

	return &role, nil
}

func (c *Client) CreateRole(reqCtx RequestContext, uuid string, role *ProvManagementRole) (*ProvManagementRole, error) {
	bodyBytes, err := json.Marshal(role)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal role body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, "/api/v1/managementRole/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusConflict {
			return nil, apperror.New(apperror.CodeConflict, "role already exists")
		}
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var created ProvManagementRole
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode created role", err)
	}

	return &created, nil
}

func (c *Client) UpdateRole(reqCtx RequestContext, uuid string, role *ProvManagementRole) (*ProvManagementRole, error) {
	bodyBytes, err := json.Marshal(role)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal role body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPut, "/api/v1/managementRole/"+uuid, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "role not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var updated ProvManagementRole
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode updated role", err)
	}

	return &updated, nil
}

func (c *Client) DeleteRole(reqCtx RequestContext, uuid string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/managementRole/"+uuid, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return apperror.New(apperror.CodeNotFound, "role not found")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

// Subscriber APIs----------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func (c *Client) ListSubscribers(reqCtx RequestContext) ([]ProvSignupEntry, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/subscriber?listOnly=true", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvSignupList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode subscriber signup list", err)
	}

	return list.Signups, nil
}

func (c *Client) ListAllEntities(reqCtx RequestContext) ([]ProvEntity, error) {
	var all []ProvEntity
	limit := 100
	offset := 0
	for i := 0; i < 1000; i++ {
		list, err := c.ListEntities(reqCtx, limit, offset)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		all = append(all, list...)
		if len(list) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *Client) ListAllVenues(reqCtx RequestContext) ([]ProvVenue, error) {
	var all []ProvVenue
	limit := 100
	offset := 0
	for i := 0; i < 1000; i++ {
		list, err := c.ListVenues(reqCtx, limit, offset)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		all = append(all, list...)
		if len(list) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *Client) ListAllPolicies(reqCtx RequestContext) ([]ProvManagementPolicy, error) {
	var all []ProvManagementPolicy
	limit := 100
	offset := 0
	for i := 0; i < 1000; i++ {
		list, err := c.ListPolicies(reqCtx, limit, offset)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		all = append(all, list...)
		if len(list) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *Client) ListAllRoles(reqCtx RequestContext) ([]ProvManagementRole, error) {
	var all []ProvManagementRole
	limit := 100
	offset := 0
	for i := 0; i < 1000; i++ {
		list, err := c.ListRoles(reqCtx, limit, offset)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		all = append(all, list...)
		if len(list) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

// Configuration APIs ----------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

type ProvConfigurationElement struct {
	Name          string      `json:"name,omitempty"`
	Description   string      `json:"description,omitempty"`
	Weight        int         `json:"weight,omitempty"`
	Configuration interface{} `json:"configuration,omitempty"`
}

type ProvConfiguration struct {
	Info          ProvObjectInfo             `json:"info"`
	DeviceTypes   []string                   `json:"deviceTypes"`
	Configuration []ProvConfigurationElement `json:"configuration"`
	Variables     []string                   `json:"variables,omitempty"`
	Entity        string                     `json:"entity,omitempty"`
	Venue         string                     `json:"venue,omitempty"`
}

type ProvConfigurationList struct {
	Configurations []ProvConfiguration `json:"configurations"`
}

func (c *Client) ListConfigurations(reqCtx RequestContext) ([]ProvConfiguration, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/configuration?limit=500&offset=0", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvConfigurationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode configurations list", err)
	}

	return list.Configurations, nil
}

func (c *Client) GetConfiguration(reqCtx RequestContext, id string) (*ProvConfiguration, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/configuration/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "configuration not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var config ProvConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode configuration", err)
	}

	return &config, nil
}

func (c *Client) CreateConfiguration(reqCtx RequestContext, id string, config interface{}) (*ProvConfiguration, error) {
	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal configuration body", err)
	}

	url := "/api/v1/configuration/0"
	if id != "" && id != "0" {
		url = "/api/v1/configuration/" + id
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var created ProvConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode created configuration", err)
	}

	return &created, nil
}

func (c *Client) DeleteConfiguration(reqCtx RequestContext, id string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/configuration/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

// Variable APIs ----------------->>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

type ProvVariableElement struct {
	Type   string `json:"type,omitempty"`
	Weight int    `json:"weight,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Value  string `json:"value,omitempty"`
}

type ProvVariableBlock struct {
	ID             string                `json:"id,omitempty"`
	Name           string                `json:"name,omitempty"`
	Description    string                `json:"description,omitempty"`
	Info           ProvObjectInfo        `json:"info,omitempty"`
	Entity         string                `json:"entity,omitempty"`
	Venue          string                `json:"venue,omitempty"`
	Configurations []string              `json:"configurations,omitempty"`
	Variables      []ProvVariableElement `json:"variables"`
	Created        int64                 `json:"created,omitempty"`
	Modified       int64                 `json:"modified,omitempty"`
}

type ProvVariableBlockList struct {
	VariableBlocks []ProvVariableBlock `json:"variableBlocks"`
	Variables      []ProvVariableBlock `json:"variables"`
}

func (c *Client) ListVariables(reqCtx RequestContext) ([]ProvVariableBlock, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/variable?limit=500&offset=0", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list ProvVariableBlockList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode variable list", err)
	}

	var result []ProvVariableBlock
	if len(list.VariableBlocks) > 0 {
		result = list.VariableBlocks
	} else {
		result = list.Variables
	}

	// Normalize ID and Name into Info if Info is empty
	for i := range result {
		if result[i].Info.ID == "" && result[i].ID != "" {
			result[i].Info.ID = result[i].ID
		}
		if result[i].Info.Name == "" && result[i].Name != "" {
			result[i].Info.Name = result[i].Name
		}
	}

	return result, nil
}

func (c *Client) GetVariable(reqCtx RequestContext, id string) (*ProvVariableBlock, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/variable/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "variable block not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var vb ProvVariableBlock
	if err := json.NewDecoder(resp.Body).Decode(&vb); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode variable block", err)
	}

	return &vb, nil
}

func (c *Client) CreateVariable(reqCtx RequestContext, id string, variablePayload interface{}) (*ProvVariableBlock, error) {
	bodyBytes, err := json.Marshal(variablePayload)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal variable payload", err)
	}

	url := "/api/v1/variable/0"
	if id != "" && id != "0" {
		url = "/api/v1/variable/" + id
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var created ProvVariableBlock
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode created variable block", err)
	}

	return &created, nil
}

func (c *Client) DeleteVariable(reqCtx RequestContext, id string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/variable/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

