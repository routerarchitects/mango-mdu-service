package services

import (
	"encoding/json"
	"testing"
)

func TestGetDefaultPolicyEntries(t *testing.T) {
	tests := []struct {
		name              string
		role              string
		userID            string
		scopeType         string
		scopeID           string
		expectedResources []string
		expectedAccess    []string
		expectedScopeType string
	}{
		{
			name:              "root role",
			role:              "root",
			userID:            "user-1",
			scopeType:         "entity",
			scopeID:           "ent-1",
			expectedResources: []string{"entity", "venue", "operator", "inventory", "configuration", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"READ", "LIST", "CREATE", "MODIFY", "DELETE"},
			expectedScopeType: "entity",
		},
		{
			name:              "admin role",
			role:              "admin",
			userID:            "user-1",
			scopeType:         "venue",
			scopeID:           "ven-1",
			expectedResources: []string{"entity", "venue", "operator", "inventory", "configuration", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"READ", "LIST", "CREATE", "MODIFY", "DELETE"},
			expectedScopeType: "venue",
		},
		{
			name:              "csr role",
			role:              "csr",
			userID:            "user-2",
			scopeType:         "entity",
			scopeID:           "ent-2",
			expectedResources: []string{"entity", "venue", "operator", "inventory", "configuration", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"READ", "LIST"},
			expectedScopeType: "entity",
		},
		{
			name:              "installer role",
			role:              "installer",
			userID:            "user-3",
			scopeType:         "entity",
			scopeID:           "ent-3",
			expectedResources: []string{"configuration", "inventory"},
			expectedAccess:    []string{"READ", "LIST", "MODIFY"},
			expectedScopeType: "entity",
		},
		{
			name:              "accounting role",
			role:              "accounting",
			userID:            "user-4",
			scopeType:         "venue",
			scopeID:           "ven-4",
			expectedResources: []string{"entity", "venue", "operator", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"READ", "LIST"},
			expectedScopeType: "venue",
		},
		{
			name:              "fallback unknown role",
			role:              "unknown-role",
			userID:            "user-5",
			scopeType:         "entity",
			scopeID:           "ent-5",
			expectedResources: []string{"entity", "venue", "operator", "inventory", "configuration", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"READ", "LIST"},
			expectedScopeType: "entity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := getDefaultPolicyEntries(tt.role, tt.userID, tt.scopeType, tt.scopeID)

			if len(entries) == 0 {
				t.Fatalf("expected non-empty default policy entries for role %s", tt.role)
			}

			// Validate the first entry (for multi-entry roles like noc, we will have another test case)
			entry := entries[0]
			if len(entry.Users) != 1 || entry.Users[0] != tt.userID {
				t.Errorf("expected user %s, got %+v", tt.userID, entry.Users)
			}

			// Compare resources
			if len(entry.Resources) != len(tt.expectedResources) {
				t.Errorf("expected resources %+v, got %+v", tt.expectedResources, entry.Resources)
			} else {
				for i, r := range entry.Resources {
					if r != tt.expectedResources[i] {
						t.Errorf("expected resource %s at index %d, got %s", tt.expectedResources[i], i, r)
					}
				}
			}

			// Compare access
			if len(entry.Access) != len(tt.expectedAccess) {
				t.Errorf("expected access %+v, got %+v", tt.expectedAccess, entry.Access)
			} else {
				for i, a := range entry.Access {
					if a != tt.expectedAccess[i] {
						t.Errorf("expected access %s at index %d, got %s", tt.expectedAccess[i], i, a)
					}
				}
			}

			// Validate policy json scope
			var scopeMap map[string]interface{}
			if err := json.Unmarshal([]byte(entry.Policy), &scopeMap); err != nil {
				t.Fatalf("failed to unmarshal policy scope JSON: %v", err)
			}

			if scopeMap["type"] != tt.expectedScopeType {
				t.Errorf("expected policy type %s, got %s", tt.expectedScopeType, scopeMap["type"])
			}

			if tt.expectedScopeType == "entity" {
				if scopeMap["entityId"] != tt.scopeID {
					t.Errorf("expected entityId %s, got %v", tt.scopeID, scopeMap["entityId"])
				}
			} else {
				if scopeMap["venueId"] != tt.scopeID {
					t.Errorf("expected venueId %s, got %v", tt.scopeID, scopeMap["venueId"])
				}
			}
		})
	}
}

func TestGetDefaultPolicyEntriesNoc(t *testing.T) {
	entries := getDefaultPolicyEntries("noc", "user-noc", "entity", "ent-noc")

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for noc, got %d", len(entries))
	}

	// First entry (structural resources)
	e1 := entries[0]
	expectedE1Resources := []string{"entity", "venue", "operator", "managementPolicy", "managementRole"}
	expectedE1Access := []string{"READ", "LIST"}
	if len(e1.Resources) != len(expectedE1Resources) {
		t.Errorf("entry 1 expected resources %+v, got %+v", expectedE1Resources, e1.Resources)
	}
	if len(e1.Access) != len(expectedE1Access) {
		t.Errorf("entry 1 expected access %+v, got %+v", expectedE1Access, e1.Access)
	}

	// Second entry (configuration/inventory write)
	e2 := entries[1]
	expectedE2Resources := []string{"configuration", "inventory"}
	expectedE2Access := []string{"READ", "LIST", "MODIFY"}
	if len(e2.Resources) != len(expectedE2Resources) {
		t.Errorf("entry 2 expected resources %+v, got %+v", expectedE2Resources, e2.Resources)
	}
	if len(e2.Access) != len(expectedE2Access) {
		t.Errorf("entry 2 expected access %+v, got %+v", expectedE2Access, e2.Access)
	}
}
