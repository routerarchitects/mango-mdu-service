package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type fakePropertyProv struct {
	tree             *models.OWProvTreeNode
	inventory        map[string][]models.InventoryTag
	inventoryErr     map[string]error
	venues           map[string]*models.VenueDetails
	updateVenueErr   error
	updateVenueCalls int
	lastUpdateBody   []byte
	mu               sync.Mutex
	treeCalls        int
	inventoryCalls   int
	delay            time.Duration
}

func (f *fakePropertyProv) GetPropertyTree(context.Context, string, string, string) (*models.OWProvTreeNode, error) {
	f.mu.Lock()
	f.treeCalls++
	f.mu.Unlock()
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.tree, nil
}

func (f *fakePropertyProv) GetInventoryForScope(_ context.Context, entityID, venueID, _, _, _ string) ([]models.InventoryTag, error) {
	f.mu.Lock()
	f.inventoryCalls++
	f.mu.Unlock()
	key := "entity:" + entityID
	if venueID != "" {
		key = "venue:" + venueID
	}
	if err := f.inventoryErr[key]; err != nil {
		return nil, err
	}
	return append([]models.InventoryTag(nil), f.inventory[key]...), nil
}

func (f *fakePropertyProv) GetVenue(_ context.Context, id, _, _, _ string) (*models.VenueDetails, error) {
	if f.venues != nil && f.venues[id] != nil {
		return f.venues[id], nil
	}
	return &models.VenueDetails{ID: id, Name: "Venue " + id}, nil
}

func (f *fakePropertyProv) UpdateVenue(_ context.Context, _ string, body []byte, _, _, _ string) error {
	f.mu.Lock()
	f.updateVenueCalls++
	f.lastUpdateBody = body
	f.mu.Unlock()
	return f.updateVenueErr
}

type fakePropertyGW struct {
	statuses    []models.GatewayDeviceStatus
	err         error
	mu          sync.Mutex
	calls       int
	inFlight    int
	maxInFlight int
	delay       time.Duration
}

func (f *fakePropertyGW) GetDevicesStatus(context.Context, []string, string, string, string) ([]models.GatewayDeviceStatus, error) {
	f.mu.Lock()
	f.calls++
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inFlight--
		f.mu.Unlock()
	}()
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return append([]models.GatewayDeviceStatus(nil), f.statuses...), f.err
}

func TestPropertyTreeFlattensAssignedScopesWrapper(t *testing.T) {
	service := NewPropertyService(&fakePropertyProv{tree: &models.OWProvTreeNode{
		Type: "entity", Name: "Assigned Scopes", UUID: "0000-0000-0000",
		Children: []models.OWProvTreeNode{{Type: "entity", Name: "Property A", UUID: "property-a"}},
		Venues:   []models.OWProvTreeNode{{Type: "venue", Name: "Venue B", UUID: "venue-b"}},
	}}, &fakePropertyGW{})

	tree, err := service.GetTree(context.Background(), "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetTree returned error: %v", err)
	}
	if len(tree.Nodes) != 2 {
		t.Fatalf("expected 2 real roots, got %d", len(tree.Nodes))
	}
	if tree.Nodes[0].Type != "property" || tree.Nodes[1].Type != "venue" {
		t.Fatalf("unexpected normalized types: %+v", tree.Nodes)
	}
	for _, node := range tree.Nodes {
		if node.ID == "0000-0000-0000" {
			t.Fatal("synthetic Assigned Scopes wrapper must not be selectable")
		}
	}
}

func TestPropertyTreeResponseOmitsDeviceSerials(t *testing.T) {
	service := NewPropertyService(&fakePropertyProv{tree: &models.OWProvTreeNode{
		Type: "entity", Name: "Property A", UUID: "property-a", Devices: []string{"sensitive-serial"},
		Venues: []models.OWProvTreeNode{{
			Type: "venue", Name: "Venue A", UUID: "venue-a", Devices: []string{"nested-sensitive-serial"},
		}},
	}}, &fakePropertyGW{})

	tree, err := service.GetTree(context.Background(), "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetTree returned error: %v", err)
	}
	payload, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal tree response: %v", err)
	}
	serialized := string(payload)
	if strings.Contains(serialized, "devices") || strings.Contains(serialized, "sensitive-serial") {
		t.Fatalf("structural tree exposed Device inventory: %s", serialized)
	}
	if len(tree.Nodes) != 1 || len(tree.Nodes[0].Devices) != 0 || len(tree.Nodes[0].Children[0].Devices) != 0 {
		t.Fatalf("structural tree must discard embedded Device serials: %+v", tree.Nodes)
	}
}

func TestPropertyPortfolioSummarizesEveryHierarchyRowInOneSnapshot(t *testing.T) {
	service := NewPropertyService(&fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "entity", Name: "Property A", UUID: "property-a",
			Venues: []models.OWProvTreeNode{{
				Type: "venue", Name: "Lobby", UUID: "venue-a",
				Children: []models.OWProvTreeNode{{Type: "venue", Name: "Reception", UUID: "venue-b"}},
			}},
		},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {{SerialNumber: "aa"}},
			"venue:venue-a":     {{SerialNumber: "bb"}},
			"venue:venue-b":     {{SerialNumber: "cc"}},
		},
	}, &fakePropertyGW{statuses: []models.GatewayDeviceStatus{
		{SerialNumber: "aa", Connected: true},
		{SerialNumber: "bb", Connected: true},
		{SerialNumber: "cc", Connected: false},
	}})

	portfolio, err := service.GetPortfolio(context.Background(), "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetPortfolio returned error: %v", err)
	}
	if portfolio.PropertyCount != 1 || portfolio.VenueCount != 2 || portfolio.NestedVenueCount != 1 {
		t.Fatalf("unexpected scope counts: %+v", portfolio)
	}
	if portfolio.Summary.TotalDevices != 3 || portfolio.Summary.OnlineDevices != 2 || portfolio.Summary.OfflineDevices != 1 {
		t.Fatalf("unexpected portfolio summary: %+v", portfolio.Summary)
	}
	root := portfolio.Nodes[0]
	if root.Summary == nil || root.Summary.TotalDevices != 3 || root.Children[0].Summary == nil || root.Children[0].Summary.TotalDevices != 2 {
		t.Fatalf("expected recursive row summaries, got %+v", root)
	}
}

func TestPropertyMapReturnsDirectDevicesWithFactualAvailability(t *testing.T) {
	healthySanity := 97.0
	lowSanity := 65.0
	service := NewPropertyService(&fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "entity", Name: "Property A", UUID: "property-a",
			Venues: []models.OWProvTreeNode{{Type: "venue", Name: "Lobby", UUID: "venue-a"}},
		},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {{SerialNumber: "property-device", Name: "Property AP"}},
			"venue:venue-a":     {{SerialNumber: "venue-device", Name: "Venue AP"}},
		},
	}, &fakePropertyGW{statuses: []models.GatewayDeviceStatus{
		{SerialNumber: "property-device", Connected: true, Sanity: &healthySanity},
		{SerialNumber: "venue-device", Connected: false, Sanity: &lowSanity},
	}})

	result, err := service.GetMap(context.Background(), "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetMap returned error: %v", err)
	}
	if len(result.Nodes) != 1 || len(result.Nodes[0].Devices) != 1 || len(result.Nodes[0].Children) != 1 {
		t.Fatalf("unexpected map hierarchy: %+v", result)
	}
	if got := result.Nodes[0].Devices[0]; got.SerialNumber != "property-device" || got.Name != "Property AP" || got.Availability != "online" || got.Sanity == nil || *got.Sanity != healthySanity {
		t.Fatalf("unexpected Property Device projection: %+v", got)
	}
	if got := result.Nodes[0].Children[0].Devices[0]; got.SerialNumber != "venue-device" || got.Name != "Venue AP" || got.Availability != "offline" || got.Sanity == nil || *got.Sanity != lowSanity {
		t.Fatalf("unexpected Venue Device projection: %+v", got)
	}
}

func TestScopedDevicesFiltersAttentionReasonsBeforePagination(t *testing.T) {
	lowSanity := 45.0
	healthySanity := 96.0
	service := NewPropertyService(&fakePropertyProv{
		tree: &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {
				{SerialNumber: "low", Name: "Low Sanity AP"},
				{SerialNumber: "missing", Name: "Missing Sanity AP"},
				{SerialNumber: "offline", Name: "Offline AP"},
				{SerialNumber: "healthy", Name: "Healthy AP"},
			},
		},
	}, &fakePropertyGW{statuses: []models.GatewayDeviceStatus{
		{SerialNumber: "low", Connected: true, Sanity: &lowSanity},
		{SerialNumber: "missing", Connected: true},
		{SerialNumber: "offline", Connected: false},
		{SerialNumber: "healthy", Connected: true, Sanity: &healthySanity},
	}})

	low, err := service.GetDevices(context.Background(), "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{PageSize: 25, Attention: "low-sanity"})
	if err != nil || low.TotalCount != 1 || low.Devices[0].SerialNumber != "low" {
		t.Fatalf("expected only the online low-sanity Device, result=%+v err=%v", low, err)
	}
	missing, err := service.GetDevices(context.Background(), "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{PageSize: 25, Attention: "sanity-unavailable"})
	if err != nil || missing.TotalCount != 1 || missing.Devices[0].SerialNumber != "missing" {
		t.Fatalf("expected only the online Device with unavailable sanity, result=%+v err=%v", missing, err)
	}
}

func TestPropertyOverviewReportsFailedStatusAsUnknown(t *testing.T) {
	service := NewPropertyService(&fakePropertyProv{
		tree: &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {{SerialNumber: "aa"}, {SerialNumber: "bb"}},
		},
	}, &fakePropertyGW{err: errors.New("owgw unavailable")})

	overview, err := service.GetOverview(context.Background(), "property", "property-a", "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetOverview returned error: %v", err)
	}
	if overview.Summary.OfflineDevices != 0 || overview.Summary.UnknownDevices != 2 {
		t.Fatalf("failed status fetch must be unknown, not offline: %+v", overview.Summary)
	}
	if !overview.DataQuality.Partial || overview.DataQuality.State != "partial" {
		t.Fatalf("expected partial data quality, got %+v", overview.DataQuality)
	}
}

func TestScopedDevicesIncludesDescendantsAndFiltersBeforePagination(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "entity", Name: "Property A", UUID: "property-a",
			Venues: []models.OWProvTreeNode{{Type: "venue", Name: "Venue A", UUID: "venue-a"}},
		},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {{SerialNumber: "aa", Name: "Lobby AP", Entity: "property-a"}},
			"venue:venue-a":     {{SerialNumber: "bb", Name: "Suite AP", Venue: "venue-a"}},
		},
	}
	gw := &fakePropertyGW{statuses: []models.GatewayDeviceStatus{
		{SerialNumber: "aa", Connected: true},
		{SerialNumber: "bb", Connected: false},
	}}
	service := NewPropertyService(prov, gw)

	result, err := service.GetDevices(context.Background(), "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{
		PageIndex: 0, PageSize: 1, Availability: "online",
	})
	if err != nil {
		t.Fatalf("GetDevices returned error: %v", err)
	}
	if result.TotalCount != 1 || len(result.Devices) != 1 || result.Devices[0].SerialNumber != "aa" {
		t.Fatalf("expected filtered online result before pagination, got %+v", result)
	}
	if result.Summary.TotalDevices != 2 {
		t.Fatalf("summary must represent complete unfiltered scope, got %+v", result.Summary)
	}
	if result.Summary.OnlineDevices != 1 || result.Summary.OfflineDevices != 1 || result.Summary.UnknownDevices != 0 {
		t.Fatalf("summary must contain factual availability counts: %+v", result.Summary)
	}
}

func TestPropertyDeviceItemReportsOfflineAvailability(t *testing.T) {
	item := propertyDeviceItem(models.InventoryTag{SerialNumber: "aa"}, map[string]models.GatewayDeviceStatus{
		"aa": {SerialNumber: "aa", Connected: false},
	})
	if item.Availability != "offline" || item.Connected || !item.StatusKnown {
		t.Fatalf("offline Device must expose only its factual availability, got %+v", item)
	}

	summary := summarize(map[string]string{"aa": "aa"}, map[string]models.GatewayDeviceStatus{
		"aa": {SerialNumber: "aa", Connected: false},
	})
	if summary.TotalDevices != 1 || summary.OfflineDevices != 1 || summary.OnlineDevices != 0 {
		t.Fatalf("scope must aggregate availability without inventing health, got %+v", summary)
	}
}

func TestScopedDevicesFiltersUnknownBeforePagination(t *testing.T) {
	service := NewPropertyService(&fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "entity", Name: "Property A", UUID: "property-a",
			Venues: []models.OWProvTreeNode{{Type: "venue", Name: "Venue A", UUID: "venue-a"}},
		},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {
				{SerialNumber: "aa", Name: "Alpha", Entity: "property-a"},
				{SerialNumber: "cc", Name: "Charlie", Entity: "property-a"},
			},
			"venue:venue-a": {{SerialNumber: "bb", Name: "Bravo", Venue: "venue-a"}},
		},
	}, &fakePropertyGW{statuses: []models.GatewayDeviceStatus{
		{SerialNumber: "aa", Connected: true},
		{SerialNumber: "bb", Connected: false},
	}})

	result, err := service.GetDevices(context.Background(), "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{
		PageIndex: 0, PageSize: 1, Availability: "unknown",
	})
	if err != nil {
		t.Fatalf("GetDevices returned error: %v", err)
	}
	if result.TotalCount != 1 || result.TotalPages != 1 || len(result.Devices) != 1 {
		t.Fatalf("expected one Unknown Device after complete-scope filtering, got %+v", result)
	}
	if result.Devices[0].SerialNumber != "cc" || result.Devices[0].Availability != "unknown" || result.Devices[0].StatusKnown {
		t.Fatalf("missing OWGW status must remain Unknown: %+v", result.Devices[0])
	}
}

func TestScopedDevicesRejectsIncompleteInventory(t *testing.T) {
	service := NewPropertyService(&fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "entity", Name: "Property A", UUID: "property-a",
			Venues: []models.OWProvTreeNode{{Type: "venue", Name: "Venue A", UUID: "venue-a"}},
		},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {{SerialNumber: "aa", Name: "Alpha", Entity: "property-a"}},
		},
		inventoryErr: map[string]error{"venue:venue-a": errors.New("inventory page unavailable")},
	}, &fakePropertyGW{})

	result, err := service.GetDevices(context.Background(), "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{
		PageIndex: 0, PageSize: 25,
	})
	if err == nil || result != nil {
		t.Fatalf("incomplete inventory must fail closed, result=%+v err=%v", result, err)
	}
}

func TestOperationalSnapshotIsReusedAcrossPropertyEndpoints(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{
			"entity:property-a": {{SerialNumber: "aa", Name: "Lobby AP"}},
		},
	}
	gw := &fakePropertyGW{statuses: []models.GatewayDeviceStatus{{SerialNumber: "aa", Connected: true}}}
	service := NewPropertyService(prov, gw)
	ctx := context.Background()

	if _, err := service.GetPortfolio(ctx, "token-a", "req", "corr"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetMap(ctx, "token-a", "req", "corr"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetOverview(ctx, "property", "property-a", "token-a", "req", "corr"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetDevices(ctx, "property", "property-a", "token-a", "req", "corr", models.ScopedDevicesQuery{PageSize: 25}); err != nil {
		t.Fatal(err)
	}

	prov.mu.Lock()
	treeCalls, inventoryCalls := prov.treeCalls, prov.inventoryCalls
	prov.mu.Unlock()
	gw.mu.Lock()
	statusCalls := gw.calls
	gw.mu.Unlock()
	if treeCalls != 1 || inventoryCalls != 1 || statusCalls != 1 {
		t.Fatalf("expected one shared snapshot load, tree=%d inventory=%d status=%d", treeCalls, inventoryCalls, statusCalls)
	}
}

func TestOperationalSnapshotSingleflightAndPrincipalIsolation(t *testing.T) {
	prov := &fakePropertyProv{
		tree:      &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{"entity:property-a": {{SerialNumber: "aa"}}},
		delay:     15 * time.Millisecond,
	}
	gw := &fakePropertyGW{
		statuses: []models.GatewayDeviceStatus{{SerialNumber: "aa", Connected: true}},
		delay:    15 * time.Millisecond,
	}
	service := NewPropertyService(prov, gw)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.GetPortfolio(context.Background(), "token-a", "req", "corr"); err != nil {
				t.Errorf("GetPortfolio: %v", err)
			}
		}()
	}
	wg.Wait()
	prov.mu.Lock()
	firstPrincipalTreeCalls := prov.treeCalls
	prov.mu.Unlock()
	gw.mu.Lock()
	firstPrincipalStatusCalls := gw.calls
	gw.mu.Unlock()
	if firstPrincipalTreeCalls != 1 || firstPrincipalStatusCalls != 1 {
		t.Fatalf("singleflight did not coalesce requests: tree=%d status=%d", firstPrincipalTreeCalls, firstPrincipalStatusCalls)
	}

	if _, err := service.GetPortfolio(context.Background(), "token-b", "req", "corr"); err != nil {
		t.Fatal(err)
	}
	prov.mu.Lock()
	totalTreeCalls := prov.treeCalls
	prov.mu.Unlock()
	gw.mu.Lock()
	totalStatusCalls := gw.calls
	gw.mu.Unlock()
	if totalTreeCalls != 2 || totalStatusCalls != 2 {
		t.Fatalf("a different principal reused another token's snapshot: tree=%d status=%d", totalTreeCalls, totalStatusCalls)
	}
}

func TestOWGWStatusConcurrencyIsBoundedAcrossPrincipals(t *testing.T) {
	const (
		principalCount = 10
		deviceCount    = 1_000
	)
	inventory := make([]models.InventoryTag, deviceCount)
	for i := range inventory {
		inventory[i] = models.InventoryTag{SerialNumber: fmt.Sprintf("device-%04d", i)}
	}
	prov := &fakePropertyProv{
		tree:      &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{"entity:property-a": inventory},
	}
	gw := &fakePropertyGW{delay: 5 * time.Millisecond}
	service := NewPropertyService(prov, gw)

	var wg sync.WaitGroup
	for i := 0; i < principalCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			token := fmt.Sprintf("token-%d", index)
			if _, err := service.GetPortfolio(context.Background(), token, "req", "corr"); err != nil {
				t.Errorf("GetPortfolio(%s): %v", token, err)
			}
		}(i)
	}
	wg.Wait()

	gw.mu.Lock()
	calls, maxInFlight := gw.calls, gw.maxInFlight
	gw.mu.Unlock()
	expectedCalls := principalCount * (deviceCount / propertyStatusBatchSize)
	if calls != expectedCalls {
		t.Fatalf("expected %d OWGW batches, got %d", expectedCalls, calls)
	}
	if maxInFlight > propertyOWGWConcurrency {
		t.Fatalf("global OWGW concurrency exceeded %d: observed %d", propertyOWGWConcurrency, maxInFlight)
	}
	if maxInFlight < 2 {
		t.Fatalf("test did not exercise concurrent OWGW calls: observed %d", maxInFlight)
	}
}

func TestKafkaStatusPatchesDoNotOverwriteAbsentFields(t *testing.T) {
	sanity := 95.0
	prov := &fakePropertyProv{
		tree:      &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{"entity:property-a": {{SerialNumber: "aa"}}},
	}
	service := NewPropertyService(prov, &fakePropertyGW{statuses: []models.GatewayDeviceStatus{{SerialNumber: "aa", Connected: true, Sanity: &sanity}}})
	ctx := context.Background()
	if _, err := service.GetMap(ctx, "token", "req", "corr"); err != nil {
		t.Fatal(err)
	}

	service.UpdateDeviceStatus("state", "aa", nil, nil, "")
	result, err := service.GetMap(ctx, "token", "req", "corr")
	if err != nil {
		t.Fatal(err)
	}
	device := result.Nodes[0].Devices[0]
	if device.Availability != "online" || device.Sanity == nil || *device.Sanity != sanity {
		t.Fatalf("state event overwrote cached availability or sanity: %+v", device)
	}

	service.UpdateDeviceStatus("healthcheck", "aa", nil, nil, "")
	result, err = service.GetMap(ctx, "token", "req", "corr")
	if err != nil {
		t.Fatal(err)
	}
	if result.Nodes[0].Devices[0].Sanity != nil {
		t.Fatalf("healthcheck without sanity must clear the cached measurement: %+v", result.Nodes[0].Devices[0])
	}
}

func TestKafkaPatchDoesNotOverwriteNewerOWGWStatus(t *testing.T) {
	prov := &fakePropertyProv{
		tree:      &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{"entity:property-a": {{SerialNumber: "aa"}}},
	}
	gw := &fakePropertyGW{
		statuses: []models.GatewayDeviceStatus{
			{SerialNumber: "aa", Connected: true, IPAddress: "192.168.1.1"},
		},
	}
	service := NewPropertyService(prov, gw)
	ctx := context.Background()

	// Initial fetch warms the cache with IP 192.168.1.1
	if _, err := service.GetDevices(ctx, "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{PageSize: 25}); err != nil {
		t.Fatal(err)
	}

	// 1. Kafka event sets IP to 192.168.1.10
	service.UpdateDeviceStatus("state", "aa", nil, nil, "192.168.1.10")
	res, err := service.GetDevices(ctx, "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{PageSize: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Devices) == 0 || res.Devices[0].IPAddress != "192.168.1.10" {
		t.Fatalf("expected Kafka IP 192.168.1.10, got %+v", res.Devices)
	}

	// 2. OWGW updates device IP to 192.168.1.20 and snapshot refreshes
	gw.mu.Lock()
	gw.statuses = []models.GatewayDeviceStatus{
		{SerialNumber: "aa", Connected: true, IPAddress: "192.168.1.20"},
	}
	gw.mu.Unlock()

	impl := service.(*propertyService)
	cacheKey := principalCacheKey("token") + ":portfolio"
	// Invalidate cache to force a fresh fetch from OWGW
	impl.cacheMu.Lock()
	delete(impl.snapshots, cacheKey)
	// Set livePatches timestamp older so it won't override the newer OWGW poll
	patch := impl.livePatches["aa"]
	patch.ipAddressAt = time.Now().Add(-10 * time.Second)
	impl.livePatches["aa"] = patch
	impl.cacheMu.Unlock()

	res, err = service.GetDevices(ctx, "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{PageSize: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Devices) == 0 || res.Devices[0].IPAddress != "192.168.1.20" {
		t.Fatalf("expected refreshed OWGW IP 192.168.1.20, got %+v", res.Devices)
	}

	// 3. New Kafka healthcheck event arrives containing ONLY sanity (sanity = 98, NO IP)
	sanityVal := 98.0
	service.UpdateDeviceStatus("healthcheck", "aa", nil, &sanityVal, "")

	// 4. Verify that sanity was updated, and the newer OWGW IP (192.168.1.20) was NOT overwritten by the old Kafka patch (192.168.1.10)
	res, err = service.GetDevices(ctx, "property", "property-a", "token", "req", "corr", models.ScopedDevicesQuery{PageSize: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Devices) == 0 {
		t.Fatal("expected at least 1 device in response")
	}
	device := res.Devices[0]
	if device.Sanity == nil || *device.Sanity != 98.0 {
		t.Fatalf("expected sanity 98.0, got %v", device.Sanity)
	}
	if device.IPAddress != "192.168.1.20" {
		t.Fatalf("CRITICAL: old Kafka patch overwrote newer OWGW IP! Expected 192.168.1.20, got %s", device.IPAddress)
	}
}

func TestPeriodicKafkaPatchCleanup(t *testing.T) {
	service := NewPropertyService(&fakePropertyProv{}, &fakePropertyGW{})
	impl := service.(*propertyService)

	// Add an old patch
	service.UpdateDeviceStatus("state", "olddevice", nil, nil, "10.0.0.1")

	// Artificially make the patch older than propertyMaxStaleAge
	impl.cacheMu.Lock()
	patch := impl.livePatches["olddevice"]
	patch.ipAddressAt = time.Now().Add(-propertyMaxStaleAge - time.Second)
	impl.livePatches["olddevice"] = patch
	// Set last cleanup older than propertyMaxStaleAge so the next Kafka event triggers cleanup
	impl.lastPatchCleanup = time.Now().Add(-propertyMaxStaleAge - time.Second)
	impl.cacheMu.Unlock()

	// New Kafka event arrives for a different device
	service.UpdateDeviceStatus("state", "newdevice", nil, nil, "10.0.0.2")

	// Verify olddevice was pruned and newdevice was retained
	impl.cacheMu.RLock()
	_, oldExists := impl.livePatches["olddevice"]
	_, newExists := impl.livePatches["newdevice"]
	cleanupTime := impl.lastPatchCleanup
	impl.cacheMu.RUnlock()

	if oldExists {
		t.Fatal("expected expired patch for olddevice to be pruned during periodic cleanup")
	}
	if !newExists {
		t.Fatal("expected newdevice patch to be stored in livePatches")
	}
	if time.Since(cleanupTime) > 5*time.Second {
		t.Fatalf("expected lastPatchCleanup to be updated to recent time, got %v", cleanupTime)
	}
}

func TestStaleSnapshotKeepsMeasurementTimeAndSurvivesFailedRefresh(t *testing.T) {
	prov := &fakePropertyProv{
		tree:      &models.OWProvTreeNode{Type: "entity", Name: "Property A", UUID: "property-a"},
		inventory: map[string][]models.InventoryTag{"entity:property-a": {{SerialNumber: "aa"}}},
	}
	gw := &fakePropertyGW{statuses: []models.GatewayDeviceStatus{{SerialNumber: "aa", Connected: true}}}
	service := NewPropertyService(prov, gw)
	ctx := context.Background()
	if _, err := service.GetMap(ctx, "token", "req", "corr"); err != nil {
		t.Fatal(err)
	}
	impl := service.(*propertyService)
	cacheKey := principalCacheKey("token") + ":portfolio"
	measuredAt := time.Now().Add(-propertyCacheTTL - time.Second).UTC().Truncate(time.Second)
	impl.cacheMu.Lock()
	impl.snapshots[cacheKey].fetchedAt = measuredAt
	impl.cacheMu.Unlock()
	gw.mu.Lock()
	gw.err = errors.New("owgw unavailable")
	beforeCalls := gw.calls
	gw.mu.Unlock()

	result, err := service.GetMap(ctx, "token", "req", "corr")
	if err != nil {
		t.Fatal(err)
	}
	if result.DataQuality.State != "stale" || result.DataQuality.AsOf != measuredAt.Format(time.RFC3339) {
		t.Fatalf("stale response did not preserve snapshot measurement time: %+v", result.DataQuality)
	}

	deadline := time.Now().Add(time.Second)
	for {
		gw.mu.Lock()
		calls := gw.calls
		gw.mu.Unlock()
		if calls > beforeCalls || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	impl.cacheMu.RLock()
	cached := impl.snapshots[cacheKey]
	if cached == nil || !cached.fetchedAt.Equal(measuredAt) || len(cached.statusErrors) != 0 {
		impl.cacheMu.RUnlock()
		t.Fatalf("failed refresh replaced the last complete stale snapshot: %+v", cached)
	}
	impl.cacheMu.RUnlock()
}

type fakePropertyAnalytics struct {
	board        *models.AnalyticsBoard
	devices      []models.AnalyticsBoardDevice
	clients      []string
	lifecycle    json.RawMessage
	createdBoard models.CreateAnalyticsBoardRequest
	createCount  int
	deletedBoard string
	deleteErr    error
}

func (f *fakePropertyAnalytics) GetBoard(_ context.Context, id, _, _, _ string) (*models.AnalyticsBoard, error) {
	if f.board != nil {
		return f.board, nil
	}
	return &models.AnalyticsBoard{
		ID:   id,
		Name: "Test Board",
		VenueList: []models.AnalyticsBoardVenue{
			{ID: "venue-1", Interval: 60, Retention: 604800, MonitorSubVenues: true},
		},
	}, nil
}

func (f *fakePropertyAnalytics) CreateBoard(_ context.Context, req models.CreateAnalyticsBoardRequest, _, _, _ string) (string, error) {
	f.createdBoard = req
	f.createCount++
	return "created-board-uuid", nil
}

func (f *fakePropertyAnalytics) UpdateBoard(_ context.Context, _ string, _ models.CreateAnalyticsBoardRequest, _, _, _ string) error {
	return nil
}

func (f *fakePropertyAnalytics) DeleteBoard(_ context.Context, id, _, _, _ string) error {
	f.deletedBoard = id
	return f.deleteErr
}

func (f *fakePropertyAnalytics) GetBoardDevices(_ context.Context, _ string, _, _, _ string) ([]models.AnalyticsBoardDevice, error) {
	return f.devices, nil
}

func (f *fakePropertyAnalytics) GetBoardTimepoints(_ context.Context, _ string, _, _ int64, _, _, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{"points":[]}`), nil
}

func (f *fakePropertyAnalytics) GetWifiClients(_ context.Context, _ string, _, _, _ string) ([]string, error) {
	return f.clients, nil
}

func (f *fakePropertyAnalytics) GetClientLifecycle(_ context.Context, _, _ string, _, _ int64, _, _ int, _, _, _ string) (json.RawMessage, error) {
	return f.lifecycle, nil
}

func TestVenueMonitoring_UnauthorizedVenueBlocked(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "Authorized Venue", UUID: "venue-authorized",
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	// Calling unauthorized venue must fail with 404
	_, err := service.GetVenueMonitoring(ctx, "venue-unauthorized", "token", "req", "corr")
	if err == nil {
		t.Fatal("expected unauthorized venue to be rejected, got nil error")
	}
	var apiErr models.ApiError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != 404 {
		t.Fatalf("expected 404 ApiError, got %v", err)
	}

	// Start on unauthorized venue must fail
	_, err = service.StartVenueMonitoring(ctx, "venue-unauthorized", models.StartVenueMonitoringRequest{}, "token", "req", "corr")
	if err == nil {
		t.Fatal("expected unauthorized venue start to be rejected, got nil error")
	}

	// Clients query on unauthorized venue must fail
	_, err = service.GetVenueMonitoringClients(ctx, "venue-unauthorized", "token", "req", "corr")
	if err == nil {
		t.Fatal("expected unauthorized venue clients to be rejected, got nil error")
	}
}

func TestVenueMonitoring_AuthorizedActiveAndInactive(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{
		devices: []models.AnalyticsBoardDevice{
			{
				SerialNumber:   "ap-1",
				Connected:      true,
				Health:         90,
				Memory:         40,
				Associations2G: 5,
				Associations5G: 10,
				Associations6G: 2,
			},
			{
				SerialNumber: "ap-2",
				Connected:    false,
				Health:       0,
				Memory:       0,
			},
		},
		clients:   []string{"00:11:22:33:44:55"},
		lifecycle: json.RawMessage(`{"entries":[{"timestamp":1234}]}`),
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	// Inactive venue (no boards attached)
	resp, err := service.GetVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Active {
		t.Fatal("expected venue monitoring to be inactive")
	}

	// Start monitoring
	startResp, err := service.StartVenueMonitoring(ctx, "venue-1", models.StartVenueMonitoringRequest{
		Interval: 60, Retention: 604800,
	}, "token", "req", "corr")
	if err != nil {
		t.Fatalf("start monitoring failed: %v", err)
	}
	if !startResp.Active || startResp.BoardID != "created-board-uuid" {
		t.Fatalf("unexpected start response: %+v", startResp)
	}

	// Active venue query with boards
	fakeProvWithBoard := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-1"}},
		},
		inventory: map[string][]models.InventoryTag{
			"venue:venue-1": {
				{SerialNumber: "ap-1"},
				{SerialNumber: "ap-2"},
			},
		},
	}
	serviceActive := NewPropertyServiceWithAnalytics(fakeProvWithBoard, gw, analytics)
	activeResp, err := serviceActive.GetVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("active monitoring fetch failed: %v", err)
	}
	if activeResp.Summary == nil || activeResp.Summary.TotalDevices != 2 || activeResp.Summary.ConnectedDevices != 1 {
		t.Fatalf("unexpected summary in active response: %+v", activeResp.Summary)
	}
	if activeResp.Summary.TotalAssociations != 17 {
		t.Fatalf("expected 17 total associations, got %d", activeResp.Summary.TotalAssociations)
	}

	// Get clients
	clients, err := serviceActive.GetVenueMonitoringClients(ctx, "venue-1", "token", "req", "corr")
	if err != nil || len(clients) != 1 || clients[0] != "00:11:22:33:44:55" {
		t.Fatalf("unexpected clients response: %v, err: %v", clients, err)
	}

	// Get client lifecycle
	lc, err := serviceActive.GetVenueClientLifecycle(ctx, "venue-1", "00:11:22:33:44:55", 0, 0, 10, 0, "token", "req", "corr")
	if err != nil || len(lc) == 0 {
		t.Fatalf("unexpected lifecycle response: %s, err: %v", string(lc), err)
	}

	// Stop monitoring
	stopResp, err := serviceActive.StopVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("stop monitoring failed: %v", err)
	}
	if stopResp.Active {
		t.Fatal("expected stopped monitoring response to have active: false")
	}
}

func TestVenueMonitoring_StartIdempotency(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-existing"}},
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{
		board: &models.AnalyticsBoard{
			ID:   "board-existing",
			Name: "My Venue",
			VenueList: []models.AnalyticsBoardVenue{
				{ID: "venue-1", Interval: 60, Retention: 604800},
			},
		},
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	// Calling StartVenueMonitoring when a board already exists should NOT create a new board.
	resp, err := service.StartVenueMonitoring(ctx, "venue-1", models.StartVenueMonitoringRequest{
		Interval: 60,
	}, "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Active || resp.BoardID != "board-existing" {
		t.Fatalf("expected existing board to be returned, got %+v", resp)
	}
	if analytics.createCount != 0 {
		t.Fatalf("expected CreateBoard not to be called, got %d calls", analytics.createCount)
	}
}

func TestVenueMonitoring_Stop_ProvUpdateFails_AnalyticsUntouched(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-1"}},
		},
		updateVenueErr: errors.New("forbidden: write access denied"),
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	// When OWPROV rejects the update, analytics deletion must NOT be called
	_, err := service.StopVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error when OWPROV update fails")
	}
	if analytics.deletedBoard != "" {
		t.Fatalf("expected analytics DeleteBoard not to be called, got %q", analytics.deletedBoard)
	}
}

func TestVenueMonitoring_Stop_AnalyticsDeleteFails_RestoresAssociation(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-1"}},
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{
		deleteErr: errors.New("owanalytics connection timeout"),
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	// Stopping monitoring when analytics delete fails must restore association in OWPROV and return error
	_, err := service.StopVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error when analytics DeleteBoard fails")
	}
	if !strings.Contains(err.Error(), "owanalytics connection timeout") {
		t.Fatalf("expected analytics timeout error, got %v", err)
	}
	if analytics.deletedBoard != "board-1" {
		t.Fatalf("expected delete attempt for board-1, got %q", analytics.deletedBoard)
	}
	if prov.updateVenueCalls != 2 {
		t.Fatalf("expected 2 UpdateVenue calls (clear and restore), got %d", prov.updateVenueCalls)
	}
	if !strings.Contains(string(prov.lastUpdateBody), "board-1") {
		t.Fatalf("expected restored boards to include 'board-1', got %s", string(prov.lastUpdateBody))
	}
}

func TestVenueMonitoring_Stop_AnalyticsClientNil_PreservesAssociation(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-1"}},
		},
	}
	gw := &fakePropertyGW{}

	// Service without analytics client
	service := NewPropertyServiceWithAnalytics(prov, gw, nil)
	ctx := context.Background()

	_, err := service.StopVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error when analytics client is nil")
	}
	var apiErr models.ApiError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != 503 {
		t.Fatalf("expected 503 Service Unavailable, got %v", err)
	}
	if prov.updateVenueCalls != 0 {
		t.Fatalf("expected OWPROV not to be modified, got %d calls", prov.updateVenueCalls)
	}
}

func TestVenueMonitoring_Stop_Success(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-1"}},
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	resp, err := service.StopVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Active {
		t.Fatal("expected monitoring to be marked inactive")
	}
	if prov.updateVenueCalls != 1 {
		t.Fatalf("expected 1 UpdateVenue call, got %d", prov.updateVenueCalls)
	}
	if analytics.deletedBoard != "board-1" {
		t.Fatalf("expected board-1 to be deleted in analytics, got %q", analytics.deletedBoard)
	}
}

func TestVenueMonitoring_StartRollbackDeleteFailureLogged(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		updateVenueErr: errors.New("owprov update failure"),
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{
		deleteErr: errors.New("owanalytics delete failure during rollback"),
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	// Start monitoring update failure returns the original error even if rollback delete fails
	_, err := service.StartVenueMonitoring(ctx, "venue-1", models.StartVenueMonitoringRequest{}, "token", "req", "corr")
	if err == nil {
		t.Fatal("expected start monitoring error when UpdateVenue fails")
	}
	if !strings.Contains(err.Error(), "owprov update failure") {
		t.Fatalf("expected owprov update failure error, got %v", err)
	}
}

func TestVenueMonitoring_ParentOnlyAdmin_UnauthorizedChildDevicesExcluded(t *testing.T) {
	// The caller's tree contains ONLY the parent venue. Unauthorized child venue is omitted.
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "Parent Venue", UUID: "venue-parent",
		},
		venues: map[string]*models.VenueDetails{
			"venue-parent": {ID: "venue-parent", Name: "Parent Venue", Boards: []string{"board-1"}},
		},
		inventory: map[string][]models.InventoryTag{
			"venue:venue-parent": {
				{SerialNumber: "parent-ap-1"},
			},
			// Note: child venue inventory exists in backend, but user is parent-only
			"venue:venue-child": {
				{SerialNumber: "child-ap-1"},
				{SerialNumber: "child-ap-2"},
			},
		},
	}
	gw := &fakePropertyGW{}
	// Analytics board collects devices recursively from parent and children
	analytics := &fakePropertyAnalytics{
		devices: []models.AnalyticsBoardDevice{
			{SerialNumber: "parent-ap-1", Connected: true, Associations2G: 5, Associations5G: 5, Health: 100, Memory: 40},
			{SerialNumber: "child-ap-1", Connected: true, Associations2G: 10, Associations5G: 10, Health: 80, Memory: 60},
			{SerialNumber: "child-ap-2", Connected: false, Associations2G: 0, Associations5G: 0, Health: 0, Memory: 0},
		},
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	resp, err := service.GetVenueMonitoring(ctx, "venue-parent", "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only parent-ap-1 should be returned to the parent-only admin
	if len(resp.Devices) != 1 || resp.Devices[0].SerialNumber != "parent-ap-1" {
		t.Fatalf("expected only parent-ap-1, got %+v", resp.Devices)
	}
	if resp.Summary == nil || resp.Summary.TotalDevices != 1 || resp.Summary.ConnectedDevices != 1 {
		t.Fatalf("expected summary to reflect only 1 authorized device, got %+v", resp.Summary)
	}
	if resp.Summary.TotalAssociations != 10 {
		t.Fatalf("expected 10 associations from parent AP, got %d", resp.Summary.TotalAssociations)
	}
}

func TestVenueMonitoring_AuthorizedParentAndChild_AllDevicesIncluded(t *testing.T) {
	// The caller's tree contains parent venue AND authorized child venue
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "Parent Venue", UUID: "venue-parent",
			Venues: []models.OWProvTreeNode{
				{Type: "venue", Name: "Child Venue", UUID: "venue-child"},
			},
		},
		venues: map[string]*models.VenueDetails{
			"venue-parent": {ID: "venue-parent", Name: "Parent Venue", Boards: []string{"board-1"}},
		},
		inventory: map[string][]models.InventoryTag{
			"venue:venue-parent": {
				{SerialNumber: "parent-ap-1"},
			},
			"venue:venue-child": {
				{SerialNumber: "child-ap-1"},
			},
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{
		devices: []models.AnalyticsBoardDevice{
			{SerialNumber: "parent-ap-1", Connected: true, Associations2G: 5, Associations5G: 5, Health: 90, Memory: 40},
			{SerialNumber: "child-ap-1", Connected: true, Associations2G: 10, Associations5G: 10, Health: 85, Memory: 50},
		},
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	resp, err := service.GetVenueMonitoring(ctx, "venue-parent", "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Devices) != 2 {
		t.Fatalf("expected 2 devices for fully authorized admin, got %d", len(resp.Devices))
	}
	if resp.Summary.TotalDevices != 2 || resp.Summary.TotalAssociations != 30 {
		t.Fatalf("unexpected summary for 2 devices: %+v", resp.Summary)
	}
}

func TestVenueMonitoring_UnmatchedDeviceSerial_Excluded(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-1"}},
		},
		inventory: map[string][]models.InventoryTag{
			"venue:venue-1": {
				{SerialNumber: "known-ap"},
			},
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{
		devices: []models.AnalyticsBoardDevice{
			{SerialNumber: "known-ap", Connected: true, Associations2G: 2},
			{SerialNumber: "unmatched-rogue-ap", Connected: true, Associations2G: 10},
		},
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	resp, err := service.GetVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Devices) != 1 || resp.Devices[0].SerialNumber != "known-ap" {
		t.Fatalf("expected only known-ap, got %+v", resp.Devices)
	}
	if resp.Summary.TotalDevices != 1 || resp.Summary.TotalAssociations != 2 {
		t.Fatalf("expected summary to exclude rogue AP, got %+v", resp.Summary)
	}
}

func TestVenueMonitoring_InventoryFailure_ReturnsErrorFailClosed(t *testing.T) {
	prov := &fakePropertyProv{
		tree: &models.OWProvTreeNode{
			Type: "venue", Name: "My Venue", UUID: "venue-1",
		},
		venues: map[string]*models.VenueDetails{
			"venue-1": {ID: "venue-1", Name: "My Venue", Boards: []string{"board-1"}},
		},
		inventoryErr: map[string]error{
			"venue:venue-1": errors.New("owprov inventory db timeout"),
		},
	}
	gw := &fakePropertyGW{}
	analytics := &fakePropertyAnalytics{
		devices: []models.AnalyticsBoardDevice{
			{SerialNumber: "ap-1", Connected: true},
		},
	}

	service := NewPropertyServiceWithAnalytics(prov, gw, analytics)
	ctx := context.Background()

	// Must fail-closed: return error rather than leaking unfiltered analytics devices
	_, err := service.GetVenueMonitoring(ctx, "venue-1", "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error when inventory retrieval fails")
	}
	if !strings.Contains(err.Error(), "owprov inventory db timeout") {
		t.Fatalf("expected inventory timeout error, got %v", err)
	}
}
