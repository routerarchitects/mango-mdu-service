# Mango MDU Service — Phase 1 Specification

## 1. Purpose

This document defines the Phase 1 specification for `mango-mdu-service`.

Phase 1 establishes MDU as the Mango-facing authenticated orchestration layer for the Mango Operator UI.
Its immediate focus is providing the live **Policy Overview API** required by the Operator UI (`Users & Access -> Policies -> Overview`), alongside standard operational system routes.

---

## 2. Phase 1 Goal & Scope

### In Scope for Phase 1:
1. **Policy Overview Orchestration**:
   - `GET /api/v1/policies/{policyId}/overview`
   - Orchestrates data across **OWPROV** (management policies, management roles, entities, venues) and **OWSEC** (user identity).
   - Computes usage summaries:
     - `usedByUsers`: count of unique users assigned to the policy.
     - `scopedAssignmentsCount`: count of management role bindings using the policy.
     - `propertiesCount`: count of unique properties (entities) linked to the policy.
     - `venuesCount`: count of unique venues linked to the policy.
   - Computes the itemized list of assigned users (`assignedUsers`):
     - User ID, display name, email.
     - Associated property name and venue scope.
2. **Operational Support & Diagnostics**:
   - `GET /livez`: Unauthenticated liveness probe on port `16010`.
   - `GET /api/v1/system`: System diagnostics with Bearer token authentication.
   - `POST /api/v1/system`: Runtime log level manipulation.
3. **Security & Transport**:
   - Inbound bearer-token validation through OWSEC (`AUTH_ENABLED=true`).
   - CORS support with automatic `OPTIONS` preflight bypass for browser compatibility.
   - Distributed request tracing: `X-Request-Id` and `X-Correlation-Id`.

### Deferred Scope (Later Milestones):
- Global Dashboard metrics & fleet telemetry (deferred).
- Property & Venue overview aggregation (deferred).
- Device operations & configuration management (deferred).
- Billing, subscriber management, and client troubleshooting (Phase 2).

---

## 3. Downstream Systems Integration

### OWSEC
- Validates bearer tokens before processing protected requests.
- Provides user directory lookups to resolve user names and email addresses.

### OWPROV
- Provides policy definitions (`GET /api/v1/managementPolicy/{id}`).
- Provides management roles (`GET /api/v1/managementRole`).
- Provides entity and venue names for resolving human-readable scope labels.

---

## 4. API Contract & Authority

The authoritative OpenAPI contract for this phase is:
**`docs/phase-1/mango-mdu-openapi.yaml`**

### Target Endpoint:
`GET /api/v1/policies/{policyId}/overview`

#### Parameters:
- `policyId` (path, string, required): UUID or name of the target management policy.
- `X-Request-Id` (header, string, optional): Request tracking UUID.
- `X-Correlation-Id` (header, string, optional): Correlation tracking UUID.

#### Response Envelope:
```json
{
  "id": "523e4567-e89b-12d3-a456-426614174000",
  "name": "adfcasdfcsacs",
  "description": "asdascd",
  "status": "Active",
  "modified": 1727610000,
  "summary": {
    "usedByUsers": 2,
    "scopedAssignmentsCount": 3,
    "propertiesCount": 1,
    "venuesCount": 2
  },
  "assignedUsers": [
    {
      "id": "usr-uuid-1",
      "name": "Default User",
      "email": "user@example.com",
      "propertyId": "ent-uuid-1",
      "propertyName": "Sunrise Apartments",
      "venueId": "ven-uuid-1",
      "venueScope": "Tower A"
    }
  ]
}
```

---

## 5. Error Handling

Normalized error responses use the standard `ApiError` format:
- `400 Bad Request`: Malformed `policyId` parameter.
- `401 Unauthorized`: Missing or invalid bearer token.
- `403 Forbidden`: Caller lacks permission.
- `404 Not Found`: Target policy does not exist.
- `503 Service Unavailable`: Downstream dependency (OWPROV / OWSEC) unreachable.
