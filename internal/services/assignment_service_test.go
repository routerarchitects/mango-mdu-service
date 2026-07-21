package services

import (
	"testing"
)

func TestGetFixedPolicyEntries(t *testing.T) {
	tests := []struct {
		name              string
		role              string
		expectedResources []string
		expectedAccess    []string
	}{
		{
			name:              "root role",
			role:              "root",
			expectedResources: []string{"entity", "venue", "device", "configuration", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"FULL"},
		},
		{
			name:              "admin role",
			role:              "admin",
			expectedResources: []string{"entity", "venue", "device", "configuration", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"FULL"},
		},
		{
			name:              "csr role",
			role:              "csr",
			expectedResources: []string{"entity", "venue", "device", "configuration"},
			expectedAccess:    []string{"READ"},
		},
		{
			name:              "installer role",
			role:              "installer",
			expectedResources: []string{"venue", "configuration"},
			expectedAccess:    []string{"READ"},
		},
		{
			name:              "fallback unknown role",
			role:              "unknown-role",
			expectedResources: []string{"entity", "venue", "device", "configuration", "managementPolicy", "managementRole"},
			expectedAccess:    []string{"READ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := getFixedPolicyEntries(tt.role)

			if len(entries) == 0 {
				t.Fatalf("expected non-empty default policy entries for role %s", tt.role)
			}

			entry := entries[0]

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
		})
	}
}

func TestGetFixedPolicyEntriesNoc(t *testing.T) {
	entries := getFixedPolicyEntries("noc")

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for noc, got %d", len(entries))
	}

	// First entry (structural resources)
	e1 := entries[0]
	expectedE1Resources := []string{"entity", "venue"}
	expectedE1Access := []string{"READ"}
	if len(e1.Resources) != len(expectedE1Resources) {
		t.Errorf("entry 1 expected resources %+v, got %+v", expectedE1Resources, e1.Resources)
	}
	if len(e1.Access) != len(expectedE1Access) {
		t.Errorf("entry 1 expected access %+v, got %+v", expectedE1Access, e1.Access)
	}

	// Second entry (configuration/device modify)
	e2 := entries[1]
	expectedE2Resources := []string{"configuration", "device"}
	expectedE2Access := []string{"READ", "MODIFY"}
	if len(e2.Resources) != len(expectedE2Resources) {
		t.Errorf("entry 2 expected resources %+v, got %+v", expectedE2Resources, e2.Resources)
	}
	if len(e2.Access) != len(expectedE2Access) {
		t.Errorf("entry 2 expected access %+v, got %+v", expectedE2Access, e2.Access)
	}
}
