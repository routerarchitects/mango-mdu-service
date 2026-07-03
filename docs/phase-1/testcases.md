# Mango MDU Service - Phase 1 API Test Cases

Scope: Phase 1 public MDU Service APIs defined in the uploaded OpenAPI contract.

This document contains the approved testcase set for the public Phase 1 MDU Service API contract.

## Global Test Assumptions

- This document covers the public client-facing OpenAPI contract only.
- Public MDU northbound APIs in this contract are bearer-authenticated unless the route explicitly overrides auth.
- `GET /livez` is unauthenticated.
- Public northbound MDU APIs in this contract do **not** require `X-API-KEY` / `x-api` from callers.
- Internal/private routes and internal authentication modes are intentionally out of scope for this testcase document.
- `X-Request-Id` and `X-Correlation-Id` are optional request headers on documented protected routes.
- All shared JSON error responses use the `ApiError` envelope:
  - `ErrorCode`
  - `ErrorDetails`
  - `ErrorDescription`
- For routes where the OpenAPI contract does not define a success response schema, this testcase document does not invent one.
- `204 No Content` responses must not include a JSON body.
- Pagination list APIs must validate:
  - `metadata.total`
  - `metadata.limit`
  - `metadata.offset`
  - default pagination behavior when params are omitted
  - empty-page and high-offset behavior
- For schema-backed success responses, tests must validate:
  - required fields present
  - enum values match exactly
  - nullable fields only where allowed
  - nested arrays/objects contain required children
  - timestamps are valid RFC3339/ISO date-time strings
- Internal service-to-service auth such as `x-api` / `X-API-KEY` belongs to integration/orchestration tests for MDU downstream calls, not as a required public request header on these northbound APIs unless the OpenAPI changes.

---

# API 1/19: Liveness Probe

```http
GET /livez
```

Purpose: Check whether the service is running and healthy.

Authentication posture:

- unauthenticated

Success response body:

- The contract defines `200 OK` with description only.
- No success response schema/body is defined in the uploaded OpenAPI.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-LIVEZ-001 | Liveness probe succeeds | `200 OK`; response body not asserted beyond contract |
| TC-LIVEZ-002 | Liveness route is unauthenticated | request without bearer credentials remains contract-valid |
| TC-LIVEZ-003 | Unsupported method is outside the documented contract surface | behavior is not specified further in this document |

---

# API 2/19: Get System Diagnostics

```http
GET /api/v1/system?command={command}
```

Purpose: Retrieve system diagnostics and metrics.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Required query parameter:

```http
command=info|resources
```

Success response body:

- The contract defines `200 OK` with description only.
- No success response schema/body is defined in the uploaded OpenAPI.

Important assertions:

- `command` is required.
- `command` must be exactly one of `info` or `resources`.
- Do not invent a JSON success body until the contract defines one.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-SYSTEM-GET-001 | Get diagnostics with `command=info` succeeds | `200 OK`; success body not asserted beyond contract |
| TC-SYSTEM-GET-002 | Get diagnostics with `command=resources` succeeds | `200 OK`; success body not asserted beyond contract |
| TC-SYSTEM-GET-003 | Missing `command` returns validation error | `400 Bad Request`; `ApiError` envelope |
| TC-SYSTEM-GET-004 | Unsupported `command` enum returns validation error | `400 Bad Request`; `ApiError` envelope |
| TC-SYSTEM-GET-005 | Missing bearer token returns unauthorized | `401 Unauthorized`; contract-defined unauthorized response |
| TC-SYSTEM-GET-006 | Caller lacks permission returns forbidden | `403 Forbidden`; contract-defined forbidden response |
| TC-SYSTEM-GET-007 | Backend unavailable returns service unavailable | `503 Service Unavailable`; contract-defined unavailable response |

---

# API 3/19: Update System Diagnostics / Log Levels

```http
POST /api/v1/system
```

Purpose: Modify diagnostics log levels or query diagnostics schema.

Required headers:

```http
Authorization: Bearer <token>
Content-Type: application/json
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Request body example:

```json
{
  "command": "setloglevel",
  "subsystems": [
    {
      "tag": "http",
      "value": "debug"
    }
  ]
}
```

Success response body:

- The contract defines `200 OK` with description only.
- No success response schema/body is defined in the uploaded OpenAPI.

Important assertions:

- `command` is required.
- `command` must be one of:
  - `setloglevel`
  - `getloglevels`
  - `getloglevelnames`
  - `getsubsystemnames`
- If `subsystems` is present, every item must include both `tag` and `value`.
- Wrong `Content-Type`, malformed JSON, and empty required body must be rejected.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-SYSTEM-POST-001 | Valid `setloglevel` request succeeds | `200 OK`; success body not asserted beyond contract |
| TC-SYSTEM-POST-002 | Valid metadata query command succeeds | `200 OK`; success body not asserted beyond contract |
| TC-SYSTEM-POST-003 | Missing `command` returns validation error | `400 Bad Request`; `ApiError` envelope |
| TC-SYSTEM-POST-004 | Invalid `command` enum returns validation error | `400 Bad Request`; `ApiError` envelope |
| TC-SYSTEM-POST-005 | `subsystems` item missing `tag` returns validation error | `400 Bad Request`; `ApiError` envelope |
| TC-SYSTEM-POST-006 | `subsystems` item missing `value` returns validation error | `400 Bad Request`; `ApiError` envelope |
| TC-SYSTEM-POST-007 | Wrong field type in `subsystems` returns validation error | `400 Bad Request`; contract-defined validation error response |
| TC-SYSTEM-POST-008 | Wrong `Content-Type` is rejected | `400 Bad Request`; contract-level bad-request response |
| TC-SYSTEM-POST-009 | Malformed JSON is rejected | `400 Bad Request`; contract-level bad-request response |
| TC-SYSTEM-POST-010 | Empty body is rejected | `400 Bad Request`; request body is required |
| TC-SYSTEM-POST-011 | Missing bearer token returns unauthorized | `401 Unauthorized`; contract-defined unauthorized response |
| TC-SYSTEM-POST-012 | Caller lacks permission returns forbidden | `403 Forbidden`; contract-defined forbidden response |
| TC-SYSTEM-POST-013 | Backend unavailable returns service unavailable | `503 Service Unavailable`; contract-defined unavailable response |

---

# API 4/19: Get Active Session and Effective Access Context

```http
GET /api/v1/session
```

Purpose: Return the authenticated caller and normalized effective access context.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Success response body example:

```json
{
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "John Doe",
    "email": "user@example.com",
    "role": "admin",
    "status": "active",
    "lastLoginAt": "2026-06-30T12:00:00Z"
  },
  "activeScope": {
    "id": "223e4567-e89b-12d3-a456-426614174000",
    "name": "Regional Operations",
    "type": "entity"
  },
  "assignments": [
    {
      "assignmentId": "523e4567-e89b-12d3-a456-426614174000",
      "scopeType": "entity",
      "scopeId": "223e4567-e89b-12d3-a456-426614174000",
      "scopeName": "Regional Operations",
      "path": [
        {
          "id": "223e4567-e89b-12d3-a456-426614174000",
          "name": "Regional Operations",
          "type": "entity"
        }
      ],
      "role": "admin",
      "managementRoleId": "623e4567-e89b-12d3-a456-426614174000",
      "managementPolicyId": "523e4567-e89b-12d3-a456-426614174000"
    }
  ],
  "permissions": {
    "hierarchy": {
      "allowed": true,
      "mode": "interactive"
    },
    "users": {
      "allowed": true,
      "mode": "interactive"
    },
    "billing": {
      "allowed": false,
      "mode": "hidden"
    },
    "configurations": {
      "allowed": true,
      "mode": "read_only"
    },
    "devices": {
      "allowed": true,
      "mode": "interactive"
    }
  }
}
```

Important assertions:

- Response matches `SessionContext`.
- Required top-level fields:
  - `user`
  - `assignments`
  - `permissions`
- `activeScope` is nullable.
- `user.id` is required.
- `user.name`, `user.email`, `user.role`, `user.status`, and `user.lastLoginAt` are optional and validated only when present.
- `user.role` must match the `RoleKey` enum exactly when present.
- `assignments[*].scopeType` must match `entity | venue`.
- `permissions.*.mode` must match `hidden | read_only | interactive`.
- `lastLoginAt` must be a valid RFC3339 / ISO date-time string when non-null.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-SESSION-001 | Session lookup succeeds with populated active scope | `200 OK`; response is contract-compatible with `SessionContext` |
| TC-SESSION-002 | Session lookup succeeds with `activeScope = null` | `200 OK`; nullable `activeScope` remains contract-valid |
| TC-SESSION-003 | Session response validates required and optional user fields correctly | `user.id` is present; optional `UserSummary` fields are validated only when returned |
| TC-SESSION-004 | Session response validates assignment collection | each assignment is contract-compatible and nested path items are valid when present |
| TC-SESSION-005 | Session response validates permissions object | each permission decision is contract-compatible with valid `allowed` and `mode` values |
| TC-SESSION-006 | Missing bearer token returns unauthorized | `401 Unauthorized`; contract-defined unauthorized response |
| TC-SESSION-007 | Caller outside allowed scope returns forbidden | `403 Forbidden`; contract-defined forbidden response |
| TC-SESSION-008 | OWSEC or PROV unavailable returns service unavailable | `503 Service Unavailable`; contract-defined unavailable response |

---

# API 5/19: Get Operator Detail

```http
GET /api/v1/operators/{operatorId}
```

Purpose: Retrieve operator details.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Success response body example:

```json
{
  "id": "323e4567-e89b-12d3-a456-426614174000",
  "name": "Acme Operator",
  "description": "Primary operator for regional services",
  "entityId": "223e4567-e89b-12d3-a456-426614174000",
  "registrationId": "REG-99210-A",
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Important assertions:

- Response matches `OperatorDetail`.
- `id` and `name` are required.
- `createdAt` and `updatedAt` are valid timestamps when returned.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-GET-OPERATOR-001 | Get operator detail succeeds | `200 OK`; response is contract-compatible with `OperatorDetail` |
| TC-GET-OPERATOR-002 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-GET-OPERATOR-003 | Caller lacks scope to read operator returns forbidden | `403 Forbidden` |
| TC-GET-OPERATOR-004 | Unknown operator returns not found | `404 Not Found`; exact `ApiError` envelope |
| TC-GET-OPERATOR-005 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 6/19: Update Operator Detail

```http
PUT /api/v1/operators/{operatorId}
```

Purpose: Update operator details.

Required headers:

```http
Authorization: Bearer <token>
Content-Type: application/json
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Request body example:

```json
{
  "name": "Acme Operator",
  "description": "Primary operator for regional services",
  "registrationId": "REG-99210-A"
}
```

Success response body:

```json
{
  "id": "323e4567-e89b-12d3-a456-426614174000",
  "name": "Acme Operator",
  "description": "Primary operator for regional services",
  "entityId": "223e4567-e89b-12d3-a456-426614174000",
  "registrationId": "REG-99210-A",
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Important assertions:

- Request body matches `UpdateOperatorRequest`.
- Wrong field types, wrong `Content-Type`, and malformed JSON must be rejected.
- The request body is required by contract, but an empty JSON object `{}` remains contract-valid unless the OpenAPI later requires at least one mutable field.
- Conflict behavior must use the shared `ConflictError` envelope.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-PUT-OPERATOR-001 | Update operator succeeds | `200 OK`; response is contract-compatible with `OperatorDetail` |
| TC-PUT-OPERATOR-002 | Empty JSON object remains contract-valid | request body `{}` remains contract-compatible with `UpdateOperatorRequest` |
| TC-PUT-OPERATOR-003 | Wrong field type is rejected | `400 Bad Request` |
| TC-PUT-OPERATOR-004 | Wrong `Content-Type` is rejected | `400 Bad Request`; contract-level bad-request response |
| TC-PUT-OPERATOR-005 | Malformed JSON is rejected | `400 Bad Request`; contract-level bad-request response |
| TC-PUT-OPERATOR-006 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-PUT-OPERATOR-007 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-PUT-OPERATOR-008 | Unknown operator returns not found | `404 Not Found` |
| TC-PUT-OPERATOR-009 | Current state conflict returns conflict | `409 Conflict`; exact shared conflict envelope |
| TC-PUT-OPERATOR-010 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 7/19: Delete Operator

```http
DELETE /api/v1/operators/{operatorId}
```

Purpose: Delete an operator.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Success response:

- `204 No Content`
- No JSON response body allowed

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-DELETE-OPERATOR-001 | Delete operator succeeds | `204 No Content`; no JSON body |
| TC-DELETE-OPERATOR-002 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-DELETE-OPERATOR-003 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-DELETE-OPERATOR-004 | Unknown operator returns not found | `404 Not Found` |
| TC-DELETE-OPERATOR-005 | Conflict prevents delete | `409 Conflict`; exact shared conflict envelope |
| TC-DELETE-OPERATOR-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 8/19: List Subscribers for an Operator

```http
GET /api/v1/operators/{operatorId}/subscribers
```

Purpose: Return a simple, unpaginated list of subscriber signup entries associated with the target operator.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Success response body example:

```json
{
  "items": [
    {
      "id": "723e4567-e89b-12d3-a456-426614174000",
      "email": "subscriber@example.com",
      "userId": "823e4567-e89b-12d3-a456-426614174000",
      "operatorId": "323e4567-e89b-12d3-a456-426614174000",
      "macAddress": "00:11:22:33:44:55",
      "serialNumber": "SN-998822",
      "status": "active",
      "registrationId": "REG-99210-A",
      "createdAt": "2026-06-30T12:00:00Z"
    }
  ]
}
```

Important assertions:

- Response matches `SubscriberListResponse`.
- Each item matches `SubscriberSignup`.
- `email`, `registrationId`, and `status` are required on each subscriber.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-SUBSCRIBERS-001 | List subscribers succeeds | `200 OK`; response is contract-compatible with `SubscriberListResponse` |
| TC-SUBSCRIBERS-002 | Empty subscriber list succeeds | `200 OK`; `items = []` |
| TC-SUBSCRIBERS-003 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-SUBSCRIBERS-004 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-SUBSCRIBERS-005 | Unknown operator returns not found | `404 Not Found` |
| TC-SUBSCRIBERS-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 9/19: Get Visible Hierarchy Tree

```http
GET /api/v1/hierarchy/tree
```

Purpose: Return the hierarchy visible to the caller.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Optional query parameter:

```http
scopeEntityId
```

Success response body shape:

```json
{
  "roots": [
    {
      "id": "<Id>",
      "type": "operator|entity|venue",
      "name": "<string>",
      "parentId": "<Id|null>",
      "path": [
        {
          "id": "<Id>",
          "type": "operator|entity|venue",
          "name": "<string>"
        }
      ],
      "selectable": true,
      "hasChildren": true,
      "children": [],
      "summary": {
        "entityCount": 0,
        "venueCount": 0,
        "userCount": 0,
        "deviceCount": 0
      }
    }
  ]
}
```

Important assertions:

- Response matches `HierarchyTreeResponse`.
- `roots` is required.
- Each node matches `HierarchyNode`.
- `type` must be exactly one of `operator`, `entity`, `venue`.
- Recursive `children` must preserve node schema.
- `summary`, when present, matches `HierarchyNodeSummary`.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-HIERARCHY-001 | Get full visible hierarchy succeeds | `200 OK`; response is contract-compatible with `HierarchyTreeResponse` |
| TC-HIERARCHY-002 | Get scoped hierarchy tree succeeds | `200 OK`; tree anchored to requested scope where applicable |
| TC-HIERARCHY-003 | Scoped tree accepts contract-valid opaque `scopeEntityId` values | query parameter is treated as generic `Id` string unless the contract adds format constraints |
| TC-HIERARCHY-004 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-HIERARCHY-005 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-HIERARCHY-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 10/19: List Entities

```http
GET /api/v1/entities
```

Purpose: Return normalized entity resources filtered to the caller's effective scope.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Optional query parameters:

```http
limit
offset
```

Success response body shape:

```json
{
  "items": [
    {
      "id": "<Id>",
      "name": "<string>",
      "parentId": "<Id|null>",
      "type": "normal|subscriber",
      "path": [
        {
          "id": "<Id>",
          "type": "operator|entity|venue",
          "name": "<string>"
        }
      ],
      "venueCount": 0,
      "userCount": 0,
      "deviceCount": 0
    }
  ],
  "metadata": {
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

Important assertions:

- `items` and `metadata` are required.
- `metadata.total`, `metadata.limit`, and `metadata.offset` are required.
- Default pagination behavior must be verified when query params are omitted.
- Every item matches `EntitySummary`.
- `type` must be exactly `normal` or `subscriber`.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-LIST-ENTITIES-001 | List entities succeeds with explicit pagination | `200 OK`; response is contract-compatible with `EntityListResponse` |
| TC-LIST-ENTITIES-002 | Default pagination applies when params are omitted | `200 OK`; `metadata.limit = 20`; `metadata.offset = 0` |
| TC-LIST-ENTITIES-003 | Empty page succeeds | `200 OK`; `items = []`; metadata remains valid |
| TC-LIST-ENTITIES-004 | High offset beyond result set returns empty page | `200 OK`; `items = []`; metadata reflects request |
| TC-LIST-ENTITIES-005 | Returned item count does not exceed `metadata.limit` | `200 OK`; count assertion passes |
| TC-LIST-ENTITIES-006 | Invalid `limit` returns bad request | `400 Bad Request` |
| TC-LIST-ENTITIES-007 | Invalid `offset` returns bad request | `400 Bad Request` |
| TC-LIST-ENTITIES-008 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-LIST-ENTITIES-009 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-LIST-ENTITIES-010 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 11/19: Create Entity

```http
POST /api/v1/entities
```

Purpose: Create a new Phase 1 entity resource.

Required headers:

```http
Authorization: Bearer <token>
Content-Type: application/json
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Request body example:

```json
{
  "name": "Regional Operations",
  "description": "Regional office entity",
  "parentEntityId": "123e4567-e89b-12d3-a456-426614174000",
  "type": "normal"
}
```

Success response body example:

```json
{
  "id": "223e4567-e89b-12d3-a456-426614174000",
  "name": "Regional Operations",
  "parentId": "123e4567-e89b-12d3-a456-426614174000",
  "type": "normal",
  "path": [
    {
      "id": "223e4567-e89b-12d3-a456-426614174000",
      "name": "Regional Operations",
      "type": "entity"
    }
  ],
  "venueCount": 12,
  "userCount": 5,
  "deviceCount": 45,
  "description": "Regional office entity",
  "operatorId": "323e4567-e89b-12d3-a456-426614174000",
  "managementPolicyId": "523e4567-e89b-12d3-a456-426614174000",
  "managementRoleIds": [
    "623e4567-e89b-12d3-a456-426614174000"
  ],
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Important assertions:

- Request matches `CreateEntityRequest`.
- `name` is required.
- `type` enum is `normal | subscriber`.
- `parentEntityId` is optional.
- Unknown enum values, wrong field types, malformed JSON, and empty body must be rejected.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-CREATE-ENTITY-001 | Create entity succeeds | `201 Created`; response is contract-compatible with `EntityDetail` |
| TC-CREATE-ENTITY-002 | Missing `name` returns validation error | `400 Bad Request`; `ApiError` envelope |
| TC-CREATE-ENTITY-003 | Invalid `type` enum returns validation error | `400 Bad Request` |
| TC-CREATE-ENTITY-004 | Wrong field type returns validation error | `400 Bad Request` |
| TC-CREATE-ENTITY-005 | Malformed JSON is rejected | `400 Bad Request` |
| TC-CREATE-ENTITY-006 | Empty body is rejected | `400 Bad Request` |
| TC-CREATE-ENTITY-007 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-CREATE-ENTITY-008 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-CREATE-ENTITY-009 | Current state conflict returns conflict | `409 Conflict`; exact shared conflict envelope |
| TC-CREATE-ENTITY-010 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 12/19: Entity Detail / Update / Delete

```http
GET /api/v1/entities/{entityId}
PUT /api/v1/entities/{entityId}
DELETE /api/v1/entities/{entityId}
```

Purpose: Retrieve, update, or delete an entity.

Request body example for update:

```json
{
  "name": "Regional Operations",
  "description": "Regional office entity"
}
```

Success response body example for get/update:

```json
{
  "id": "223e4567-e89b-12d3-a456-426614174000",
  "name": "Regional Operations",
  "parentId": "123e4567-e89b-12d3-a456-426614174000",
  "type": "normal",
  "path": [
    {
      "id": "223e4567-e89b-12d3-a456-426614174000",
      "name": "Regional Operations",
      "type": "entity"
    }
  ],
  "venueCount": 12,
  "userCount": 5,
  "deviceCount": 45,
  "description": "Regional office entity",
  "operatorId": "323e4567-e89b-12d3-a456-426614174000",
  "managementPolicyId": "523e4567-e89b-12d3-a456-426614174000",
  "managementRoleIds": [
    "623e4567-e89b-12d3-a456-426614174000"
  ],
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Important assertions:

- Response matches `EntityDetail`.
- `parentId` may be null.
- `managementRoleIds`, when present, is an array of IDs.
- timestamps are valid.
- Delete success is `204 No Content` with no JSON body.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-GET-ENTITY-001 | Get entity detail succeeds | `200 OK`; response is contract-compatible with `EntityDetail` |
| TC-GET-ENTITY-002 | Entity detail with `parentId = null` succeeds | `200 OK`; nullable `parentId` remains contract-valid |
| TC-GET-ENTITY-003 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-GET-ENTITY-004 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-GET-ENTITY-005 | Unknown entity returns not found | `404 Not Found` |
| TC-GET-ENTITY-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-PUT-ENTITY-001 | Update entity succeeds | `200 OK`; response is contract-compatible with `EntityDetail` |
| TC-PUT-ENTITY-002 | Wrong field type returns validation error | `400 Bad Request` |
| TC-PUT-ENTITY-003 | Empty JSON object remains contract-valid | request body `{}` remains contract-compatible with `UpdateEntityRequest` |
| TC-PUT-ENTITY-004 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-PUT-ENTITY-005 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-PUT-ENTITY-006 | Unknown entity returns not found | `404 Not Found` |
| TC-PUT-ENTITY-007 | State conflict returns conflict | `409 Conflict` |
| TC-PUT-ENTITY-008 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-DELETE-ENTITY-001 | Delete entity succeeds | `204 No Content`; no JSON body |
| TC-DELETE-ENTITY-002 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-DELETE-ENTITY-003 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-DELETE-ENTITY-004 | Unknown entity returns not found | `404 Not Found` |
| TC-DELETE-ENTITY-005 | Conflict prevents delete | `409 Conflict` |
| TC-DELETE-ENTITY-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 13/19: List / Create Venues Under an Entity

```http
GET /api/v1/entities/{entityId}/venues
POST /api/v1/entities/{entityId}/venues
```

Purpose: List venues under an entity, and create a new venue under that entity.

Required headers for both routes:

```http
Authorization: Bearer <token>
```

Additional required header for create:

```http
Content-Type: application/json
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Optional query parameters for list:

```http
limit
offset
```

Create request body example:

```json
{
  "name": "Grand MDU Complex",
  "description": "Main residential complex venue",
  "parentVenueId": "523e4567-e89b-12d3-a456-426614174111"
}
```

Success response body example for create:

```json
{
  "id": "423e4567-e89b-12d3-a456-426614174000",
  "name": "Grand MDU Complex",
  "entityId": "223e4567-e89b-12d3-a456-426614174000",
  "parentVenueId": null,
  "path": [
    {
      "id": "223e4567-e89b-12d3-a456-426614174000",
      "name": "Regional Operations",
      "type": "entity"
    }
  ],
  "deviceCount": 45,
  "description": "Main residential complex venue",
  "managementPolicyId": "523e4567-e89b-12d3-a456-426614174000",
  "managementRoleIds": [
    "623e4567-e89b-12d3-a456-426614174000"
  ],
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Success response body shape for list:

```json
{
  "items": [
    {
      "id": "<Id>",
      "name": "<string>",
      "entityId": "<Id>",
      "parentVenueId": "<Id|null>",
      "path": [
        {
          "id": "<Id>",
          "type": "operator|entity|venue",
          "name": "<string>"
        }
      ],
      "deviceCount": 0
    }
  ],
  "metadata": {
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

Important assertions:

- List wrapper matches `VenueListResponse`.
- Create success response matches `VenueDetail`.
- Pagination exactness is validated on list route.
- `name` is required for create.
- Wrong field types, malformed JSON, and empty body are rejected on create.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-LIST-VENUES-001 | List venues under entity succeeds | `200 OK`; response is contract-compatible with `VenueListResponse` |
| TC-LIST-VENUES-002 | Default pagination applies when omitted | `metadata.limit = 20`; `metadata.offset = 0` |
| TC-LIST-VENUES-003 | Empty page succeeds | `200 OK`; `items = []` |
| TC-LIST-VENUES-004 | High offset returns empty page | `200 OK`; metadata valid |
| TC-LIST-VENUES-005 | Entity path parameter accepts contract-valid opaque IDs | path parameter is treated as generic `Id` string unless the contract adds format constraints |
| TC-LIST-VENUES-006 | Unknown entity returns not found | `404 Not Found` |
| TC-LIST-VENUES-007 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-LIST-VENUES-008 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-LIST-VENUES-009 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-CREATE-VENUE-001 | Create venue succeeds | `201 Created`; response is contract-compatible with `VenueDetail` |
| TC-CREATE-VENUE-002 | Missing `name` returns validation error | `400 Bad Request` |
| TC-CREATE-VENUE-003 | Wrong field type returns validation error | `400 Bad Request` |
| TC-CREATE-VENUE-004 | Malformed JSON is rejected | `400 Bad Request` |
| TC-CREATE-VENUE-005 | Empty body is rejected | `400 Bad Request` |
| TC-CREATE-VENUE-006 | Unknown entity returns not found | `404 Not Found` |
| TC-CREATE-VENUE-007 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-CREATE-VENUE-008 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-CREATE-VENUE-009 | Conflict returns conflict | `409 Conflict` |
| TC-CREATE-VENUE-010 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 14/19: Venue Detail / Update / Delete

```http
GET /api/v1/venues/{venueId}
PUT /api/v1/venues/{venueId}
DELETE /api/v1/venues/{venueId}
```

Purpose: Retrieve, update, or delete a venue.

Request body example for update:

```json
{
  "name": "Grand MDU Complex",
  "description": "Main residential complex venue"
}
```

Success response body example for get/update:

```json
{
  "id": "423e4567-e89b-12d3-a456-426614174000",
  "name": "Grand MDU Complex",
  "entityId": "223e4567-e89b-12d3-a456-426614174000",
  "parentVenueId": null,
  "path": [
    {
      "id": "223e4567-e89b-12d3-a456-426614174000",
      "name": "Regional Operations",
      "type": "entity"
    }
  ],
  "deviceCount": 45,
  "description": "Main residential complex venue",
  "managementPolicyId": "523e4567-e89b-12d3-a456-426614174000",
  "managementRoleIds": [
    "623e4567-e89b-12d3-a456-426614174000"
  ],
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Important assertions:

- GET/PUT response matches `VenueDetail`.
- `parentVenueId` is nullable.
- Delete success is `204 No Content` with no JSON body.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-GET-VENUE-001 | Get venue succeeds | `200 OK`; response is contract-compatible with `VenueDetail` |
| TC-GET-VENUE-002 | Nullable `parentVenueId` remains contract-valid | `200 OK` |
| TC-GET-VENUE-003 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-GET-VENUE-004 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-GET-VENUE-005 | Unknown venue returns not found | `404 Not Found` |
| TC-GET-VENUE-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-PUT-VENUE-001 | Update venue succeeds | `200 OK`; response is contract-compatible with `VenueDetail` |
| TC-PUT-VENUE-002 | Wrong field type returns validation error | `400 Bad Request` |
| TC-PUT-VENUE-003 | Empty JSON object remains contract-valid | request body `{}` remains contract-compatible with `UpdateVenueRequest` |
| TC-PUT-VENUE-004 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-PUT-VENUE-005 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-PUT-VENUE-006 | Unknown venue returns not found | `404 Not Found` |
| TC-PUT-VENUE-007 | Conflict returns conflict | `409 Conflict` |
| TC-PUT-VENUE-008 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-DELETE-VENUE-001 | Delete venue succeeds | `204 No Content`; no JSON body |
| TC-DELETE-VENUE-002 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-DELETE-VENUE-003 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-DELETE-VENUE-004 | Unknown venue returns not found | `404 Not Found` |
| TC-DELETE-VENUE-005 | Conflict returns conflict | `409 Conflict` |
| TC-DELETE-VENUE-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 15/19: Management Policies

```http
GET /api/v1/policies
POST /api/v1/policies
GET /api/v1/policies/{policyId}
PUT /api/v1/policies/{policyId}
DELETE /api/v1/policies/{policyId}
```

Purpose: List, create, retrieve, update, and delete management policies.

Optional query parameters for list:

```http
limit
offset
entityId
venueId
```

Create request body example:

```json
{
  "name": "InstallerPolicy",
  "description": "Read/Write configuration policy for installers",
  "entries": [
    {
      "users": [
        "123e4567-e89b-12d3-a456-426614174000"
      ],
      "resources": [
        "configuration",
        "inventory"
      ],
      "access": [
        "READ",
        "MODIFY"
      ]
    }
  ],
  "entity": "223e4567-e89b-12d3-a456-426614174000"
}
```

Venue-scoped request body example:

```json
{
  "name": "Venue Installer",
  "description": "Venue-scoped installation technician role",
  "managementPolicy": "523e4567-e89b-12d3-a456-426614174000",
  "users": [
    "123e4567-e89b-12d3-a456-426614174000"
  ],
  "venue": "423e4567-e89b-12d3-a456-426614174000"
}
```

Success response body example for create/get/update:

```json
{
  "id": "523e4567-e89b-12d3-a456-426614174000",
  "name": "InstallerPolicy",
  "description": "Read/Write configuration policy for installers",
  "entries": [
    {
      "users": [
        "123e4567-e89b-12d3-a456-426614174000"
      ],
      "resources": [
        "configuration",
        "inventory"
      ],
      "access": [
        "READ",
        "MODIFY"
      ]
    }
  ],
  "entity": "223e4567-e89b-12d3-a456-426614174000",
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Success response body shape for list:

```json
{
  "items": [
    {
      "id": "<Id>",
      "name": "<string>",
      "description": "<string>",
      "entries": [
        {
          "users": [
            "<Id>"
          ],
          "resources": [
            "<string>"
          ],
          "access": [
            "<PolicyAccessKey>"
          ]
        }
      ],
      "entity": "<Id>",
      "venue": "<Id>",
      "createdAt": "<IsoDateTime>",
      "updatedAt": "<IsoDateTime>"
    }
  ],
  "metadata": {
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

Important assertions:

- List wrapper matches `ManagementPolicyListResponse`.
- Item/detail response matches `ManagementPolicy`.
- `name` is required on create.
- Nested `entries[*]` must contain required `resources` and `access`.
- Nested `entries[*].users`, when present, must be accepted as an array of IDs and preserved in returned `ManagementPolicy` payloads.
- Nested enum values in `access` must exactly match `PolicyAccessKey`.
- Exact pagination behavior is validated on list route.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-LIST-POLICIES-001 | List policies succeeds | `200 OK`; response is contract-compatible with `ManagementPolicyListResponse` |
| TC-LIST-POLICIES-002 | Default pagination applies when omitted | metadata defaults verified |
| TC-LIST-POLICIES-003 | High offset returns empty page | `200 OK`; empty page valid |
| TC-LIST-POLICIES-004 | Invalid pagination params return bad request | `400 Bad Request` |
| TC-LIST-POLICIES-005 | Filter by `entityId` succeeds | `200 OK`; returned policies satisfy filter |
| TC-LIST-POLICIES-006 | Filter by `venueId` succeeds | `200 OK`; returned policies satisfy filter |
| TC-LIST-POLICIES-007 | List policies preserves `entries[*].users` when present | `200 OK`; returned items include the same `users` arrays |
| TC-LIST-POLICIES-008 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-LIST-POLICIES-009 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-LIST-POLICIES-010 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-CREATE-POLICY-001 | Create policy succeeds | `201 Created`; response is contract-compatible with `ManagementPolicy` |
| TC-CREATE-POLICY-002 | Create policy preserves `entries[*].users` when provided | `201 Created`; returned `ManagementPolicy.entries[*].users` matches request |
| TC-CREATE-POLICY-003 | Missing `name` returns validation error | `400 Bad Request` |
| TC-CREATE-POLICY-004 | Invalid nested `entries` shape returns validation error | `400 Bad Request` |
| TC-CREATE-POLICY-005 | Invalid `access` enum returns validation error | `400 Bad Request` |
| TC-CREATE-POLICY-006 | Wrong field type returns validation error | `400 Bad Request` |
| TC-CREATE-POLICY-007 | Malformed JSON or empty body is rejected | `400 Bad Request` |
| TC-CREATE-POLICY-008 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-CREATE-POLICY-009 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-CREATE-POLICY-010 | Conflict returns conflict | `409 Conflict` |
| TC-CREATE-POLICY-011 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-GET-POLICY-001 | Get policy detail succeeds | `200 OK`; response is contract-compatible with `ManagementPolicy` |
| TC-GET-POLICY-002 | Get policy detail preserves `entries[*].users` when present | `200 OK`; returned `ManagementPolicy.entries[*].users` matches persisted data |
| TC-GET-POLICY-003 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-GET-POLICY-004 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-GET-POLICY-005 | Unknown policy returns not found | `404 Not Found` |
| TC-GET-POLICY-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-PUT-POLICY-001 | Update policy succeeds | `200 OK`; response is contract-compatible with `ManagementPolicy` |
| TC-PUT-POLICY-002 | Update policy preserves `entries[*].users` when provided | `200 OK`; returned `ManagementPolicy.entries[*].users` matches request |
| TC-PUT-POLICY-003 | Invalid nested payload returns validation error | `400 Bad Request` |
| TC-PUT-POLICY-004 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-PUT-POLICY-005 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-PUT-POLICY-006 | Unknown policy returns not found | `404 Not Found` |
| TC-PUT-POLICY-007 | Conflict returns conflict | `409 Conflict` |
| TC-PUT-POLICY-008 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-DELETE-POLICY-001 | Delete policy succeeds | `204 No Content`; no JSON body |
| TC-DELETE-POLICY-002 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-DELETE-POLICY-003 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-DELETE-POLICY-004 | Unknown policy returns not found | `404 Not Found` |
| TC-DELETE-POLICY-005 | Conflict returns conflict | `409 Conflict` |
| TC-DELETE-POLICY-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 16/19: Management Roles

```http
GET /api/v1/roles
POST /api/v1/roles
GET /api/v1/roles/{roleId}
PUT /api/v1/roles/{roleId}
DELETE /api/v1/roles/{roleId}
```

Purpose: List, create, retrieve, update, and delete management roles.

Optional query parameters for list:

```http
limit
offset
entityId
```

Create request body example:

```json
{
  "name": "Installer",
  "description": "On-site installation technician role",
  "managementPolicy": "523e4567-e89b-12d3-a456-426614174000",
  "users": [
    "123e4567-e89b-12d3-a456-426614174000"
  ],
  "entity": "223e4567-e89b-12d3-a456-426614174000"
}
```

Success response body example for create/get/update:

```json
{
  "id": "623e4567-e89b-12d3-a456-426614174000",
  "name": "Installer",
  "description": "On-site installation technician role",
  "managementPolicy": "523e4567-e89b-12d3-a456-426614174000",
  "users": [
    "123e4567-e89b-12d3-a456-426614174000"
  ],
  "entity": "223e4567-e89b-12d3-a456-426614174000",
  "createdAt": "2026-06-30T12:00:00Z",
  "updatedAt": "2026-06-30T12:00:00Z"
}
```

Success response body shape for list:

```json
{
  "items": [
    {
      "id": "<Id>",
      "name": "<string>",
      "description": "<string>",
      "managementPolicy": "<Id>",
      "users": [
        "<Id>"
      ],
      "entity": "<Id>",
      "venue": "<Id>",
      "createdAt": "<IsoDateTime>",
      "updatedAt": "<IsoDateTime>"
    }
  ],
  "metadata": {
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

Important assertions:

- List wrapper matches `ManagementRoleListResponse`.
- Item/detail response matches `ManagementRole`.
- `name` and `managementPolicy` are required on create.
- `users`, when present, must be an array of IDs.
- Role payloads may be entity-scoped or venue-scoped, and either scope must remain contract-compatible when returned.
- Exact pagination behavior is validated on list route.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-LIST-ROLES-001 | List roles succeeds | `200 OK`; response is contract-compatible with `ManagementRoleListResponse` |
| TC-LIST-ROLES-002 | Default pagination applies when params are omitted | metadata defaults verified |
| TC-LIST-ROLES-003 | High offset returns empty page | `200 OK`; empty page valid |
| TC-LIST-ROLES-004 | Invalid pagination params return bad request | `400 Bad Request` |
| TC-LIST-ROLES-005 | Filter by `entityId` succeeds | `200 OK`; returned roles satisfy filter |
| TC-LIST-ROLES-006 | Venue-scoped role items remain contract-compatible when returned | `200 OK`; returned venue-scoped items preserve the `venue` field |
| TC-LIST-ROLES-007 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-LIST-ROLES-008 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-LIST-ROLES-009 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-CREATE-ROLE-001 | Create role succeeds | `201 Created`; response is contract-compatible with `ManagementRole` |
| TC-CREATE-ROLE-002 | Venue-scoped role create succeeds | `201 Created`; response preserves the venue-scoped `ManagementRole` shape |
| TC-CREATE-ROLE-003 | Missing `name` returns validation error | `400 Bad Request` |
| TC-CREATE-ROLE-004 | Missing `managementPolicy` returns validation error | `400 Bad Request` |
| TC-CREATE-ROLE-005 | Wrong type in `users` returns validation error | `400 Bad Request` |
| TC-CREATE-ROLE-006 | Malformed JSON or empty body is rejected | `400 Bad Request` |
| TC-CREATE-ROLE-007 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-CREATE-ROLE-008 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-CREATE-ROLE-009 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-GET-ROLE-001 | Get role detail succeeds | `200 OK`; response is contract-compatible with `ManagementRole` |
| TC-GET-ROLE-002 | Venue-scoped role detail remains contract-compatible when returned | `200 OK`; returned role preserves the `venue` field |
| TC-GET-ROLE-003 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-GET-ROLE-004 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-GET-ROLE-005 | Unknown role returns not found | `404 Not Found` |
| TC-GET-ROLE-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-PUT-ROLE-001 | Update role succeeds | `200 OK`; response is contract-compatible with `ManagementRole` |
| TC-PUT-ROLE-002 | Venue-scoped role update succeeds | `200 OK`; response preserves the venue-scoped `ManagementRole` shape |
| TC-PUT-ROLE-003 | Wrong field type returns validation error | `400 Bad Request` |
| TC-PUT-ROLE-004 | Empty JSON object remains contract-valid | request body `{}` remains contract-compatible with `UpdateManagementRoleRequest` |
| TC-PUT-ROLE-005 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-PUT-ROLE-006 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-PUT-ROLE-007 | Unknown role returns not found | `404 Not Found` |
| TC-PUT-ROLE-008 | Conflict returns conflict | `409 Conflict` |
| TC-PUT-ROLE-009 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-DELETE-ROLE-001 | Delete role succeeds | `204 No Content`; no JSON body |
| TC-DELETE-ROLE-002 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-DELETE-ROLE-003 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-DELETE-ROLE-004 | Unknown role returns not found | `404 Not Found` |
| TC-DELETE-ROLE-005 | Conflict returns conflict | `409 Conflict` |
| TC-DELETE-ROLE-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 17/19: Get Effective Scoped Assignments for a User

```http
GET /api/v1/users/{userId}/assignments
```

Purpose: Return the target user's effective scoped assignments.

Required headers:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Success response body shape:

```json
{
  "items": [
    {
      "assignmentId": "<Id>",
      "scopeType": "entity|venue",
      "scopeId": "<Id>",
      "scopeName": "<string>",
      "role": "root|admin|csr|installer|noc|accounting|system",
      "path": [
        {
          "id": "<Id>",
          "type": "operator|entity|venue",
          "name": "<string>"
        }
      ],
      "managementRoleId": "<Id>",
      "managementPolicyId": "<Id>",
      "createdAt": "<IsoDateTime>"
    }
  ]
}
```

Important assertions:

- Wrapper contains required `items`.
- Every item matches `UserAssignment`.
- `scopeType` is exactly `entity | venue`.
- `role` matches `RoleKey`.
- `createdAt` is a valid timestamp when returned.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-GET-ASSIGNMENTS-001 | Get user assignments succeeds | `200 OK`; response is contract-compatible with `UserAssignmentListResponse` |
| TC-GET-ASSIGNMENTS-002 | Empty assignment list succeeds | `200 OK`; `items = []` |
| TC-GET-ASSIGNMENTS-003 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-GET-ASSIGNMENTS-004 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-GET-ASSIGNMENTS-005 | Unknown user returns not found | `404 Not Found` |
| TC-GET-ASSIGNMENTS-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 18/19: Create / Delete User Assignment

```http
POST /api/v1/users/{userId}/assignments
DELETE /api/v1/users/{userId}/assignments/{assignmentId}
```

Purpose: Assign a user to an entity or venue scope, or remove a scoped assignment.

Required headers for create:

```http
Authorization: Bearer <token>
Content-Type: application/json
```

Required headers for delete:

```http
Authorization: Bearer <token>
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

Request body example:

```json
{
  "scopeType": "entity",
  "scopeId": "223e4567-e89b-12d3-a456-426614174000",
  "role": "admin"
}
```

Success response body example for `200 OK` no-op / already assigned:

```json
{
  "assignmentId": "523e4567-e89b-12d3-a456-426614174000",
  "scopeType": "entity",
  "scopeId": "223e4567-e89b-12d3-a456-426614174000",
  "scopeName": "Regional Operations",
  "role": "admin",
  "path": [
    {
      "id": "223e4567-e89b-12d3-a456-426614174000",
      "name": "Regional Operations",
      "type": "entity"
    }
  ],
  "managementRoleId": "623e4567-e89b-12d3-a456-426614174000",
  "managementPolicyId": "523e4567-e89b-12d3-a456-426614174000",
  "createdAt": "2026-06-30T12:00:00Z"
}
```

Success response body example for `201 Created` created/resolved:

```json
{
  "assignmentId": "723e4567-e89b-12d3-a456-426614174000",
  "scopeType": "venue",
  "scopeId": "323e4567-e89b-12d3-a456-426614174000",
  "scopeName": "East Wing Venue",
  "role": "installer",
  "path": [
    {
      "id": "223e4567-e89b-12d3-a456-426614174000",
      "name": "Regional Operations",
      "type": "entity"
    },
    {
      "id": "323e4567-e89b-12d3-a456-426614174000",
      "name": "East Wing Venue",
      "type": "venue"
    }
  ],
  "managementRoleId": "823e4567-e89b-12d3-a456-426614174000",
  "managementPolicyId": "723e4567-e89b-12d3-a456-426614174000",
  "createdAt": "2026-06-30T12:00:00Z"
}
```

Endpoint-specific `400` example:

```json
{
  "ErrorCode": 400,
  "ErrorDetails": "The field 'scopeType' must be either 'entity' or 'venue'.",
  "ErrorDescription": "Bad Request"
}
```

Endpoint-specific `409` example:

```json
{
  "ErrorCode": 409,
  "ErrorDetails": "Cannot safely resolve requested assignment from existing downstream state.",
  "ErrorDescription": "Conflict"
}
```

Important assertions:

- Request matches `CreateUserAssignmentRequest`.
- `scopeType` must be exactly `entity | venue`.
- `role` must match `RoleKey`.
- Success body matches `UserAssignment`.
- Delete success is `204 No Content` with no JSON body.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-CREATE-ASSIGNMENT-001 | Create new scoped assignment succeeds | `201 Created`; response is contract-compatible with `UserAssignment` |
| TC-CREATE-ASSIGNMENT-002 | Existing matching role is resolved by adding user | `201 Created`; response matches `UserAssignment` |
| TC-CREATE-ASSIGNMENT-003 | Already-assigned request returns idempotent success | `200 OK`; response matches `UserAssignment` |
| TC-CREATE-ASSIGNMENT-004 | Invalid `scopeType` returns endpoint-specific validation error | `400 Bad Request`; contract-defined validation error response for this route |
| TC-CREATE-ASSIGNMENT-005 | Missing required field returns validation error | `400 Bad Request` |
| TC-CREATE-ASSIGNMENT-006 | Wrong field type returns validation error | `400 Bad Request` |
| TC-CREATE-ASSIGNMENT-007 | Malformed JSON or empty body is rejected | `400 Bad Request` |
| TC-CREATE-ASSIGNMENT-008 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-CREATE-ASSIGNMENT-009 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-CREATE-ASSIGNMENT-010 | Unknown user or scope returns not found | `404 Not Found` |
| TC-CREATE-ASSIGNMENT-011 | Unresolvable downstream state returns endpoint-specific conflict | `409 Conflict`; contract-defined conflict response for this route |
| TC-CREATE-ASSIGNMENT-012 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-DELETE-ASSIGNMENT-001 | Delete assignment succeeds | `204 No Content`; no JSON body |
| TC-DELETE-ASSIGNMENT-002 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-DELETE-ASSIGNMENT-003 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-DELETE-ASSIGNMENT-004 | Unknown assignment returns not found | `404 Not Found` |
| TC-DELETE-ASSIGNMENT-005 | Conflict returns conflict | `409 Conflict` |
| TC-DELETE-ASSIGNMENT-006 | Backend unavailable returns service unavailable | `503 Service Unavailable` |

---

# API 19/19: Get / Update User Access Policy

```http
GET /api/v1/users/{userId}/access-policy
PUT /api/v1/users/{userId}/access-policy
```

Purpose: Retrieve or update the access policy for a user at a given scope.

Required headers for both routes:

```http
Authorization: Bearer <token>
```

Additional required header for update:

```http
Content-Type: application/json
```

Optional headers:

```http
X-Request-Id
X-Correlation-Id
```

GET request shape:

```http
GET /api/v1/users/{userId}/access-policy?scope={entity|venue}&entityId={entityId}[&venueId={venueId}]
```

Entity-scope success response example:

```json
{
  "scope": "entity",
  "entityId": "223e4567-e89b-12d3-a456-426614174000",
  "roleTemplate": "Installer",
  "resourcePermissions": [
    {
      "resource": "inventory",
      "policies": [
        "READ",
        "MODIFY"
      ]
    },
    {
      "resource": "configuration",
      "policies": [
        "READ"
      ]
    }
  ]
}
```

Venue-scope success response example:

```json
{
  "scope": "venue",
  "entityId": "223e4567-e89b-12d3-a456-426614174000",
  "venueId": "423e4567-e89b-12d3-a456-426614174000",
  "roleTemplate": "Installer",
  "resourcePermissions": [
    {
      "resource": "inventory",
      "policies": [
        "READ",
        "MODIFY"
      ]
    }
  ]
}
```

Endpoint-specific `400` examples:

```json
{
  "ErrorCode": 400,
  "ErrorDetails": "The required query parameter 'entityId' is missing.",
  "ErrorDescription": "Bad Request"
}
```

```json
{
  "ErrorCode": 400,
  "ErrorDetails": "The query parameter 'venueId' is required when scope is set to 'venue'.",
  "ErrorDescription": "Bad Request"
}
```

Endpoint-specific `404` example:

```json
{
  "ErrorCode": 404,
  "ErrorDetails": "The access policy configuration for the target user at the specified scope does not exist.",
  "ErrorDescription": "Not Found"
}
```

Important assertions:

- Response must match exactly one `UserAccessPolicy` branch:
  - `EntityUserAccessPolicy`
  - `VenueUserAccessPolicy`
- Entity-scope response must contain:
  - `scope = entity`
  - `entityId`
  - `roleTemplate`
  - `resourcePermissions`
- Venue-scope response must contain:
  - `scope = venue`
  - `entityId`
  - `venueId`
  - `roleTemplate`
  - `resourcePermissions`
- `resourcePermissions[*].resource` and `policies[*]` must match declared enums exactly.
- GET negative coverage includes the documented query-parameter errors for missing `entityId`, missing venue-scope `venueId`, and invalid `scope` enum values.
- Update request must conform to exactly one `oneOf` branch.
- Entity payload must not include `venueId`.
- Venue payload must include `venueId`.

## Test Cases

| ID | Name | Expected Result |
|---|---|---|
| TC-GET-ACCESS-POLICY-001 | Get entity-scope access policy succeeds | `200 OK`; response matches exactly the entity branch of `oneOf` |
| TC-GET-ACCESS-POLICY-002 | Get venue-scope access policy succeeds | `200 OK`; response matches exactly the venue branch of `oneOf` |
| TC-GET-ACCESS-POLICY-003 | Missing `entityId` returns endpoint-specific validation error | `400 Bad Request`; contract-defined validation error response for this route |
| TC-GET-ACCESS-POLICY-004 | `scope=venue` without `venueId` returns endpoint-specific validation error | `400 Bad Request`; contract-defined validation error response for this route |
| TC-GET-ACCESS-POLICY-005 | Invalid `scope` enum returns validation error | `400 Bad Request` |
| TC-GET-ACCESS-POLICY-006 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-GET-ACCESS-POLICY-007 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-GET-ACCESS-POLICY-008 | User or binding not found returns not found | `404 Not Found` |
| TC-GET-ACCESS-POLICY-009 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
| TC-PUT-ACCESS-POLICY-001 | Update entity-scope access policy succeeds | `200 OK`; response matches entity branch of `oneOf` exactly |
| TC-PUT-ACCESS-POLICY-002 | Update venue-scope access policy succeeds | `200 OK`; response matches venue branch of `oneOf` exactly |
| TC-PUT-ACCESS-POLICY-003 | Entity-scope payload containing `venueId` returns endpoint-specific validation error | `400 Bad Request`; contract-defined validation error response for this route |
| TC-PUT-ACCESS-POLICY-004 | Venue-scope payload missing `venueId` returns validation error | `400 Bad Request` |
| TC-PUT-ACCESS-POLICY-005 | Invalid resource enum returns validation error | `400 Bad Request` |
| TC-PUT-ACCESS-POLICY-006 | Invalid policy enum returns validation error | `400 Bad Request` |
| TC-PUT-ACCESS-POLICY-007 | Malformed JSON or empty body is rejected | `400 Bad Request` |
| TC-PUT-ACCESS-POLICY-008 | Missing bearer token returns unauthorized | `401 Unauthorized` |
| TC-PUT-ACCESS-POLICY-009 | Caller lacks permission returns forbidden | `403 Forbidden` |
| TC-PUT-ACCESS-POLICY-010 | Missing downstream binding returns endpoint-specific not found | `404 Not Found`; contract-defined not-found response for this route |
| TC-PUT-ACCESS-POLICY-011 | Conflict returns shared conflict envelope | `409 Conflict`; exact `ApiError` structure |
| TC-PUT-ACCESS-POLICY-012 | Backend unavailable returns service unavailable | `503 Service Unavailable` |
