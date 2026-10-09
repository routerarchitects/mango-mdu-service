# Mango MDU Service — Phase 1 Specification

## 1. Purpose

This document defines the Phase 1 specification for `mango-mdu-service`.

Phase 1 establishes MDU as the Mango-facing authenticated orchestration layer for the Mango Operator UI.
Its immediate focus is providing the live **Policy Overview API** required by the Operator UI (`Users & Access -> Policies -> Overview`), alongside standard operational system routes.

---

## 2. Phase 1 Goal & Scope

### In Scope for Phase 1:
1. **Policy Overview Orchestration**:
   - `GET /api/v1/policy/{id}/overview`
   - Orchestrates data across **OWPROV** (management policies, management roles, entities, venues) and **OWSEC** (user identity).
   - Computes usage summaries:
     - `totalUsers`: count of unique users assigned to the policy.
     - `totalScopedAssignments`: count of management role bindings using the policy.
     - `totalProperties`: count of unique properties (entities) linked to the policy.
     - `totalVenues`: count of unique venues linked to the policy.
   - Computes the itemized list of assigned users (`usersWithPolicy`):
     - Unique user identity: `id`, `name`, `email`, `userRole`, optional `avatar`.
     - `scopedAssignmentsCount`: count of assignments for this user under this policy.
     - Grouped list of scoped assignments (`scopes[]`): each containing `entityId`, `entityName`, `venueId`, and `venueName` ("All venues" or specific venue name).
2. **Operational Support & Diagnostics**:
   - `GET /livez`: Unauthenticated liveness probe on port `16010`.
   - `GET /api/v1/system`: System diagnostics with Bearer token or `X-API-KEY` authentication (supports `command=info` returning system metadata and certificates, and `command=resources` returning memory and file descriptor metrics). Unauthenticated requests receive a bare HTTP 401 status code from common public auth middleware.
   - `POST /api/v1/system`: Runtime log level manipulation and diagnostics queries (supports `setloglevel`, `getloglevels`, `getloglevelnames`, `getsubsystemnames`).
3. **Security & Transport**:
   - Inbound bearer-token validation through OWSEC (`AUTH_ENABLED=true`) via `Authorization: Bearer <owsec-token>`.
   - Outbound downstream calls: forwards the caller's bearer token (`Authorization: Bearer <owsec-token>`) to downstream services (OWPROV and OWSEC) so they authoritatively enforce user-level RBAC and tenant scoping. For OWPROV calls, MDU identifies via client `User-Agent: mango-mdu-service/1.0` and distributed tracing headers without `X-API-KEY` (which would override user identity in OWPROV). For private OWSEC directory calls (`/api/v1/users` on `:17001`), MDU uses dual authentication (`X-INTERNAL-NAME` and `X-API-KEY` for machine identity, alongside `Authorization: Bearer <owsec-token>` for delegated user RBAC scoping).
   - CORS support with automatic `OPTIONS` preflight bypass for browser compatibility. *(Implementation note: The existing middleware currently permits standard headers; updating `RegisterPublicCORS` in `internal/http/middleware/middleware.go` to include `"X-Request-Id"` and `"X-Correlation-Id"` in `AllowHeaders` is an acceptance requirement that will be implemented during code implementation).*
   - Distributed request tracing: `X-Request-Id` and `X-Correlation-Id`.

### Deferred Scope (Later Milestones):
- Global Dashboard metrics & fleet telemetry (deferred).
- Property & Venue overview aggregation (deferred).
- Device operations & configuration management (deferred).
- Billing, subscriber management, and client troubleshooting (Phase 2).

---

## 3. Downstream Systems Integration

### Downstream Authentication & Propagation Contract
Downstream calls from MDU forward the authenticated end-user context to allow downstream services (OWPROV and OWSEC) to natively evaluate caller permissions, tenant boundaries, and entity/venue scoping.

Because OWPROV treats `X-API-KEY` as a primary authentication credential that overrides the `Authorization: Bearer` token when present, MDU **does not send `X-API-KEY` to OWPROV** on user-scoped operations. Instead, MDU relies on user token delegation with standard service attribution (`User-Agent`) and distributed tracing:

#### To OWPROV (Policy, Roles, Entities, Venues)
OWPROV evaluates the forwarded user token to enforce RBAC tenant boundaries (Entity/Venue scoping):
- **`Authorization: Bearer <owsec-token>`**: Primary auth carrying end-user context for downstream RBAC and scope evaluation.
- **`User-Agent: mango-mdu-service/1.0`**: Identifies MDU as the originating calling service in downstream access logs.
- **`X-Request-Id` & `X-Correlation-Id`**: Distributed request tracing headers.

```http
Authorization: Bearer <owsec-token>
User-Agent: mango-mdu-service/1.0
X-Request-Id: <request-id>
X-Correlation-Id: <correlation-id>
```

#### To OWSEC (Token Validation & User Profile Lookups)
OWSEC validates authentication tokens and provides user directory lookups (`GET /api/v1/users` over private endpoint `:17001`). For private user directory calls, MDU uses dual authentication:
- **`X-INTERNAL-NAME`**: Identifies `mango-mdu-service` as an authentic internal OpenWiFi microservice.
- **`X-API-KEY`**: Authenticates MDU's internal machine identity to the private router.
- **`Authorization: Bearer <owsec-token>`**: Preserves the original end-user token, enabling OWSEC to enforce caller RBAC and multi-tenant scoping (`createdby` filtering).
- **`User-Agent: mango-mdu-service/1.0`**: Identifies MDU service in downstream access logs.
- **`X-Request-Id` & `X-Correlation-Id`**: Distributed request tracing headers.

```http
X-INTERNAL-NAME: <mdu-private-endpoint-or-name>
X-API-KEY: <mdu-service-api-key>
Authorization: Bearer <owsec-token>
User-Agent: mango-mdu-service/1.0
X-Request-Id: <request-id>
X-Correlation-Id: <correlation-id>
```

### OWSEC
- Validates bearer tokens before processing protected requests.
- Provides bulk user directory lookups (`GET /api/v1/users` with pagination) to resolve user profiles (names, emails, avatars) accessible to the authenticated requester.
- **Requester-Scoped User Mapping & Unresolved User Handling**:
  - To avoid an $N+1$ HTTP request bottleneck of querying each user individually when resolving large role sets, MDU retrieves the requester's accessible users via paginated bulk retrieval (`GET /api/v1/users?offset={offset}&limit={limit}`) and indexes them in an in-memory lookup map.
  - Roles with empty or unassigned users (`role.users == []`) are counted towards `totalScopedAssignments`, but contribute 0 users to `usersWithPolicy`.
  - When mapping roles to users, if a user UUID is not present in the requester's accessible user set (e.g., user was deleted from OWSEC, or is outside the caller's administrative scope), MDU **simply skips** that user. MDU does **not** synthesize artificial placeholder records (`"Deleted User"`).
  - `totalUsers` authoritatively reflects the count of unique, accessible users assigned to this policy, naturally maintaining `usersWithPolicy.length == totalUsers` without injecting false data into the UI.
  - (Note: In a later lifecycle milestone, role deletion/cleanup will be coordinated in OWPROV when users are removed).

### OWPROV
- Provides policy definitions (`GET /api/v1/managementPolicy/{id}`).
- Provides management roles (`GET /api/v1/managementRole`). Evaluates user RBAC and entity/venue scoping based on the forwarded user token. Leverages the native `policyId` query filter (`GET /api/v1/managementRole?policyId={id}`) to perform database-level filtering directly in OWPROV. If a policy has a large volume of roles exceeding OWPROV's page size, MDU paginates using `limit=500` and `offset` until all policy-scoped roles are retrieved, preventing silent metric truncation while avoiding full-table scans.
- Provides entity and venue names (`GET /api/v1/entity`, `GET /api/v1/venue`) for resolving human-readable scope labels.

---

## 4. API Contract & Authority

The authoritative OpenAPI contract for this phase is:
**`docs/phase-1/mango-mdu-openapi.yaml`**

### Target Endpoint:
`GET /api/v1/policy/{id}/overview`

#### Parameters:
- `id` (path, string, required): UUID of the target management policy.
- `X-Request-Id` (header, string, optional): Request tracking UUID.
- `X-Correlation-Id` (header, string, optional): Correlation tracking UUID.

#### Response Envelope:
```json
{
  "policy": {
    "id": "523e4567-e89b-12d3-a456-426614174000",
    "name": "Network Operator",
    "description": "Monitor devices and manage network configuration.",
    "entity": "",
    "venue": "",
    "created": 1725000000,
    "modified": 1725500000
  },
  "totalUsers": 9,
  "totalScopedAssignments": 14,
  "totalProperties": 4,
  "totalVenues": 12,
  "usersWithPolicy": [
    {
      "id": "4b96f542-2304-4c15-b19e-f43d21d93c50",
      "name": "Anita Sharma",
      "email": "anita@ipnx.example",
      "userRole": "noc",
      "avatar": "1",
      "scopedAssignmentsCount": 2,
      "scopes": [
        {
          "entityId": "e290f1ee-6c54-4b01-90e6-d701748f0851",
          "entityName": "Sunrise Apartments",
          "venueId": "",
          "venueName": "All venues"
        },
        {
          "entityId": "f81d4fae-7dec-11d0-a765-00a0c91e6bf6",
          "entityName": "Oakwood Housing",
          "venueId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
          "venueName": "Building A"
        }
      ]
    }
  ]
}
```

---

## 5. Error Handling

Normalized error responses adhere to the standard OpenWiFi/MDU PascalCase `ApiError` envelope defined in `docs/phase-1/mango-mdu-openapi.yaml`:

```json
{
  "ErrorCode": 404,
  "ErrorDescription": "Not Found",
  "ErrorDetails": "Management policy not found"
}
```

### Schema Fields:
- `ErrorCode` (integer, required): HTTP status code or normalized application error code.
- `ErrorDescription` (string, required): Standard HTTP status description or high-level error summary.
- `ErrorDetails` (string, optional): Context-specific diagnostic message explaining the cause when available.

### Standard Error Statuses:
- `400 Bad Request`: Malformed `id` parameter (`ErrorCode: 400`).
- `401 Unauthorized`: Missing or invalid authentication (`ErrorCode: 401` on MDU endpoints; bare HTTP 401 status on shared `/api/v1/system`).
- `403 Forbidden`: Caller lacks permission (`ErrorCode: 403`).
- `404 Not Found`: Target policy does not exist (`ErrorCode: 404`).
- `503 Service Unavailable`: Downstream dependency (OWPROV / OWSEC) unreachable (`ErrorCode: 503`).
