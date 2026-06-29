# Mango MDU Service — Phase 1 Workflow

## Purpose

This document describes how Phase 1 of `mango-mdu-service` works at the workflow level.

It is based on the Phase 1 scope from the MDU master requirements and Phase 1 specification:

- MDU is the Mango-facing authenticated orchestration layer
- OWSEC is used for token validation and identity/session checks
- PROV is used for foundational operator-domain workflows
- MDU does not own login, session issuance, or users (OWSEC is the authoritative owner); customers, hierarchy, roles, policies, and RBAC truth remain with PROV
- MDU normalizes Mango-facing contracts and hides downstream quirks

This workflow document explains the intended runtime flow of Phase 1 business APIs and how MDU interacts with OWSEC and PROV.

**Document usage note:** This document describes the intended Phase 1 runtime and orchestration behavior. It is a workflow document, not the final API contract document or a statement that every Phase 1 business route is already implemented in the current runtime. Endpoint contract details, request/response schemas, and full normalized error definitions belong in the Phase 1 spec and OpenAPI.

---

## Core Phase 1 Principle

Phase 1 is **not** just local MDU APIs.

Phase 1 is:

1. UI authenticates with OWSEC
2. UI calls MDU with OWSEC bearer token
3. MDU validates the token
4. MDU calls PROV using service auth plus forwarded user context
5. PROV evaluates business access and returns source-of-truth data
6. MDU shapes the final Mango-facing response

So Phase 1 is already a real integration phase with **OWSEC + PROV**.

---

## Main Phase 1 API Surface

Phase 1 includes the following Mango-facing API families defined in `mango-mdu-openapi.yaml`:

- **Session Context:** `GET /mdu/me`
- **Hierarchy Tree:** `GET /mdu/hierarchy/tree`
- **Entities Management:** `GET /mdu/entities`, `POST /mdu/entities`, `GET /mdu/entities/{entityId}`, `PATCH /mdu/entities/{entityId}`, `DELETE /mdu/entities/{entityId}`
- **Venues Management:** `GET /mdu/entities/{entityId}/venues`, `POST /mdu/entities/{entityId}/venues`, `GET /mdu/venues/{venueId}`, `PATCH /mdu/venues/{venueId}`, `DELETE /mdu/venues/{venueId}`
- **User Scoped Assignments:** `GET /mdu/users/{userId}/entities`, `POST /mdu/users/{userId}/entities`, `DELETE /mdu/users/{userId}/entities/{entityId}`

In the current runtime baseline, the checked-in support routes are:

- `GET /livez`
- `/api/v1/system`

The `/mdu/*` sections below describe the intended Phase 1 business workflow model to be implemented.

---

## Known PROV Route Families Used in Phase 1

MDU orchestrates downstream calls using these key PROV route families:

- **Entity Paths:** `GET /entity`, `POST /entity`, `GET /entity/{uuid}`, `PUT /entity/{uuid}`, `DELETE /entity/{uuid}`
- **Venue Paths:** `GET /venue`, `POST /venue`, `GET /venue/{uuid}`, `PUT /venue/{uuid}`, `DELETE /venue/{uuid}`
- **RBAC & Delegation Paths:** `GET /managementPolicy`, `GET /managementRole` (and their respective /{uuid} detail lookups)

---

## Common Headers in Phase 1

### Inbound to MDU

Protected Phase 1 APIs receive:

```http
Authorization: Bearer <owsec-token>
X-Request-Id: <request-id>         optional
X-Correlation-Id: <correlation-id> optional
```

If request or correlation IDs are missing, MDU generates them.

### Outbound from MDU to PROV

For user-context PROV workflows, MDU sends:

```http
x-api: <mdu-service-api-key>
x-authorization: Bearer <owsec-token>
x-request-id: <request-id>
x-correlation-id: <correlation-id>
```

This is the standard Phase 1 downstream trust flow.

---

# 1. Common Phase 1 Request Workflow

Every protected Phase 1 business API follows the same base workflow.

## Step 1 — UI calls MDU

The Mango UI or equivalent caller sends a request to an MDU Phase 1 endpoint under `/api/v1/mdu/*`.

## Step 2 — MDU validates transport input

MDU parses:

- route and method
- path parameters
- query parameters
- request body if present
- `Authorization`
- request/correlation IDs

If the request is malformed, MDU returns a normalized validation error without calling downstream services.

## Step 3 — MDU validates bearer token through OWSEC

Before protected business execution, MDU validates the bearer token using OWSEC-owned validation behavior.

Validation confirms at minimum:

- token is present
- token is valid
- token is not expired
- caller identity is usable for the target workflow

If validation fails, MDU returns a normalized auth failure and does not continue to PROV.

## Step 4 — MDU determines target workflow

Once the token is valid, MDU determines which Phase 1 workflow is being executed:

- Session bootstrap (`/mdu/me`)
- Hierarchy tree (`/mdu/hierarchy/tree`)
- Entities management
- Venues management
- User scoped assignments
- Billing workflows

## Step 5 — MDU calls PROV where domain truth is required

If the workflow depends on PROV-owned data or authorization, MDU calls PROV with:

- service authentication (`x-api`)
- forwarded user context (`x-authorization`)
- trace headers (`x-request-id`, `x-correlation-id`)

## Step 6 — PROV evaluates access and returns truth

PROV remains responsible for:

- scope decisions
- RBAC decisions
- hierarchy visibility
- source-of-truth data for entities, venues, and user assignments

OWSEC is the authoritative owner for user identity (user CRUD, login, token issuance). PROV provides the user's Mango operational context: operator scope, customer scope, roles, policies, and hierarchy visibility.

## Step 7 — MDU normalizes response

MDU maps the downstream response into a Mango-facing contract.

This is where MDU adds value:

- normalizes field shape
- hides downstream quirks
- maps errors into stable MDU categories
- returns a clean MDU response model

## Step 8 — MDU returns response

The caller receives a normalized Phase 1 response.

---

# 2. Bootstrap Workflow — `GET /mdu/me`

## Goal

Return the normalized caller context for the authenticated user, composed from OWSEC-validated identity and PROV-fetched operational context.

## Step-by-Step Runtime Workflow

### Step 1 — Inbound API Request
A client sends a request to the MDU bootstrap endpoint:
`GET /mdu/me`
The request carries the active OWSEC bearer token:
`Authorization: Bearer <owsec-token>`

### Step 2 — MDU validates request and Auth Token
1. Validates the request format and headers.
2. Validates the bearer token against OWSEC to ensure it is active. If validation fails, MDU returns a normalized auth error (e.g., `401 Unauthorized`).

### Step 3 — MDU fetches operational context from PROV
Once authorized, MDU forwards the caller's token (`x-authorization: Bearer <owsec-token>`) and its service credentials (`x-api`) to PROV:
1. **Fetch User Details:** MDU requests the user profile and role details.
2. **Fetch User Assignments:** MDU requests the scope permissions, role mappings, and policy bounds bound to the user.
3. **Fetch Visible Entities:** MDU retrieves the list of entities and venues that the user's role has permission to access.

### Step 4 — MDU composes SessionContext Response Body
MDU builds the response conforming to the `SessionContext` schema in `mango-mdu-openapi.yaml`:
1. **User Summary (`user`):** Populates the caller's user details including `id`, `name`, `email`, `role`, `status`, and `lastLoginAt`.
2. **Active Scope (`activeScope`):** If the user is operating within a specific entity context, MDU sets this to a `ScopePathItem` (with `id`, `type`, and `name`). Otherwise, it returns `null`.
3. **Session Assignments (`assignments`):** Merges the PROV-provided assignments into a list of `SessionAssignment` objects. For each assignment, it populates `assignmentId`, `scopeType` (`operator`, `entity`, `venue`), `scopeId`, `scopeName`, `role`, `managementRoleId`, `managementPolicyId`, and the ancestry `path` array.
4. **Permissions (`permissions`):** Evaluates the user's roles and policies against the requested scope to return an `EffectivePermissionSet`. This contains `RbacDecision` objects (`allowed: boolean`, `mode: hidden | read_only | interactive`) for each functional domain: `hierarchy`, `users`, `configurations`, and `devices`.

### Step 5 — MDU returns SessionContext response
MDU returns the composed `SessionContext` payload with a `200 OK` status.

---

# 3. Read Workflow — PROV-backed List or Detail APIs

This section describes the workflow for all read endpoints exposed by MDU that fetch entity and venue data from PROV:
- **Entities Read:** `GET /mdu/entities` and `GET /mdu/entities/{entityId}`
- **Venues Read:** `GET /mdu/entities/{entityId}/venues` and `GET /mdu/venues/{venueId}`

## Step-by-Step Runtime Workflow

### Step 1 — Inbound API Request
A client sends a read request to one of the MDU endpoints (e.g., `GET /mdu/entities` with optional `parentEntityId` query filter, or `GET /mdu/entities/{entityId}`).
The request carries the active OWSEC bearer token in the headers:
`Authorization: Bearer <owsec-token>`

### Step 2 — MDU validates request and Auth Token
1. Validates the request format, path UUID parameters, and query filters.
2. Validates the bearer token against OWSEC. If validation fails, MDU returns a normalized auth error (e.g., `401 Unauthorized`).

### Step 3 — MDU delegates call to PROV
MDU forwards the query to PROV, appending the service credential (`x-api`) and user context (`x-authorization: Bearer <owsec-token>`):
- For `GET /mdu/entities`: MDU queries PROV's `/entity` endpoint (passing filters).
- For `GET /mdu/entities/{entityId}`: MDU queries PROV's `/entity/{uuid}`.
- For `GET /mdu/entities/{entityId}/venues`: MDU queries PROV's `/venue` filtered by entity.
- For `GET /mdu/venues/{venueId}`: MDU queries PROV's `/venue/{uuid}`.

### Step 4 — MDU maps responses to OpenAPI schemas
MDU shapes the response to match the northbound contract:
1. **Entities List response (`EntityListResponse`):** Returns `items: array of EntitySummary`, where each item includes `id`, `name`, `parentId`, `type` (`normal` | `subscriber`), `path` (ancestry path array), `venueCount`, `userCount`, and `deviceCount`.
2. **Entity Detail response (`EntityDetail`):** Inherits from `EntitySummary` and adds `description`, `operatorId`, `managementPolicyId`, `managementRoleIds`, `createdAt`, and `updatedAt`.
3. **Venues List response (`VenueListResponse`):** Returns `items: array of VenueSummary`, where each item includes `id`, `name`, `entityId`, `parentVenueId`, `path` (ancestry path array), and `deviceCount`.
4. **Venue Detail response (`VenueDetail`):** Inherits from `VenueSummary` and adds `description`, `managementPolicyId`, `managementRoleIds`, `createdAt`, and `updatedAt`.

*Note: In all responses, `deviceCount` is returned as `0` since live device runtime data is out of scope for Phase 1.*

### Step 5 — MDU returns shaped response
MDU returns the JSON payload with a `200 OK` status.

---

# 4. Hierarchy Workflow — Entities, Venues, and Tree-Oriented Reads

Phase 1 includes foundational hierarchy-related navigation and node details backed by PROV.

## Goal

Allow MDU to expose the combined hierarchy tree and node overview data to northbound clients while keeping PROV as the authoritative owner of the hierarchy tree.

## Phase 1 Scope & Boundary Guardrails

- **In-Scope (Active):** Retrieval and structure of hierarchical nodes (Operator, Customer, Site, Building, Tower, Floor, Venue) and node summary metadata (Node Type, Scope Path, and child count metrics).
- **Out-of-Scope (Deferred in Phase 1):** Topology data, device lists, and device configuration details. Since device runtime, topology computation, and configuration assignments live in OWGW (or are deferred to later phases), these properties are returned as empty, zero, or omitted fields in the backend response.

## Step-by-Step Runtime Workflow

### Step 1 — Inbound API Request
A client sends a request to fetch the hierarchy tree:
`GET /api/v1/mdu/hierarchy/tree` (or `/api/v1/mdu/entities` when querying child nodes).
The request carries the active OWSEC bearer token in the headers:
`Authorization: Bearer <owsec-token>`

### Step 2 — MDU validates request and Auth Token
MDU performs standard validation:
1. Checks request parameters, correlation headers, and token presence.
2. Validates the bearer token against OWSEC to ensure the session is active. If validation fails, MDU returns a normalized auth error (e.g., `401 Unauthorized`).

### Step 3 — MDU orchestrates calls to downstream PROV APIs
Once authorized, MDU calls PROV to retrieve the entities and venues using either of the following PROV API patterns depending on configuration/available endpoints:
- **Option A (Tree-Capable Endpoint):**
  Call PROV's tree-oriented endpoint:
  `GET /api/v1/entity?getTree=true`
- **Option B (List and Extended Info):**
  Call PROV's bulk entity and inventory endpoints:
  `GET /api/v1/entity?withExtendedInfo=true&offset=0&limit=500`
- **Venue Enrichment:**
  To build the complete hierarchy down to individual venues (such as *Venue Lounge*, *Venue Office*), MDU calls:
  `GET /api/v1/venue` or fetches venues scoped to the retrieved entities.

MDU authenticates as a trusted service to PROV using its service credential (`x-api`) and forwards the caller's JWT (`x-authorization: Bearer <owsec-token>`) so PROV can apply visibility, tenant scope, and RBAC rules.

### Step 4 — MDU combines responses to build tree
MDU processes and combines the data returned from downstream calls to build a list of `HierarchyNode` items, complying with the `HierarchyTreeResponse` schema defined in `mango-mdu-openapi.yaml`:
1. **Combine Entities and Venues:** Merges downstream PROV entities and venues into a single unified set of hierarchy tree nodes.
2. **Build parentId Links:** Maps each node to its respective parent (by resolving parent entity/venue UUIDs).
3. **Populate Path Array:** Generates the required `path` array containing the ordered list of `ScopePathItem` ancestors from the root down to the parent of the current node (e.g., `[ {id: "uuid-1", type: "operator", name: "OpCo"}, {id: "uuid-2", type: "entity", name: "Customer West"} ]`).
4. **Determine selectable and hasChildren:** Calculates boolean flags indicating whether the node can be selected by the current user and if it has child sub-entities or venues.
5. **Populate Children Nested Tree:** Recursively nests child `HierarchyNode` objects under their parents.

### Step 5 — MDU maps counts and enforces Phase 1 exclusions
MDU calculates and populates the `HierarchyNodeSummary` for each node:
- **In-Scope Counts:** `entityCount` and `venueCount` are computed by walking the combined tree. `userCount` is fetched/mapped from downstream.
- **Phase 1 Boundary Exclusions:** Since live device runtime, configuration, and topology details live in OWGW or are deferred to later phases, the `deviceCount` is returned as `0` in the summary block.

### Step 6 — MDU returns shaped Hierarchy Tree response
MDU returns a JSON payload matching the `HierarchyTreeResponse` schema, containing a `roots` array of root-level `HierarchyNode` objects.

## Important Rule

Phase 1 does **not** create a local hierarchy source of truth inside MDU. All parent-child and venue associations remain owned by PROV.

---

# 5. Scope-constrained Read Workflow

All read operations are constrained by the user's resolved scope and delegation permissions. The caller must only be allowed to see:
- Allowed entities (Operators, Customers, Sites, Buildings, etc.)
- Allowed venues
- Assigned users, roles, and policies within their scope

## Step-by-Step Enforcement Workflow

1. **Inbound Request:** Client requests a resource or a list.
2. **Auth Token Check:** MDU validates the user context via OWSEC.
3. **Context Delegation:** MDU forwards the user context to PROV using `x-authorization`.
4. **Visibility Assessment:** PROV evaluates:
   - Resource-level ownership and association.
   - Scope-level boundaries (the calling user's tenant root entity).
   - Role-based capabilities.
5. **Shaped Results:** MDU processes the response:
   - If authorized, MDU returns the list or resource detail shaped to the MDU schema.
   - If unauthorized, MDU returns a normalized `403 Forbidden` or `404 Not Found` response.

MDU must enforce these scope boundaries at the API gateway/handler level and must not return out-of-scope records.

---

# 6. Mutation Workflow — PROV-backed Create, Update, and Delete APIs

MDU acts as an orchestrator for mutations on PROV-owned domains. The active mutations in Phase 1 are:
- **Entities Mutation:** `POST /mdu/entities` (CreateEntityRequest), `PATCH /mdu/entities/{entityId}` (UpdateEntityRequest), `DELETE /mdu/entities/{entityId}`
- **Venues Mutation:** `POST /mdu/entities/{entityId}/venues` (CreateVenueRequest), `PATCH /mdu/venues/{venueId}` (UpdateVenueRequest), `DELETE /mdu/venues/{venueId}`

## Step-by-Step Runtime Workflow

### Step 1 — Inbound mutation request
A client sends a request to mutate a resource (e.g., `POST /mdu/entities` with a JSON payload in the body).
The request carries the active OWSEC bearer token:
`Authorization: Bearer <owsec-token>`

### Step 2 — MDU validates request body and Auth Token
1. Validates the request format, UUID path parameters, and required request body schemas (e.g., `CreateEntityRequest`, `UpdateEntityRequest`, or `CreateVenueRequest`).
2. Validates the bearer token against OWSEC. If validation fails, MDU returns a normalized auth error (e.g., `401 Unauthorized`).

### Step 3 — MDU forwards mutation request to PROV
MDU constructs the downstream request to PROV, applying service credentials (`x-api`) and the user context (`x-authorization`):
- **Create Entity:** MDU sends `POST /entity` to PROV.
- **Update Entity:** MDU sends `PUT /entity/{uuid}` to PROV.
- **Delete Entity:** MDU sends `DELETE /entity/{uuid}` to PROV.
- **Create Venue:** MDU sends `POST /venue` to PROV.
- **Update Venue:** MDU sends `PUT /venue/{uuid}` to PROV.
- **Delete Venue:** MDU sends `DELETE /venue/{uuid}` to PROV.

### Step 4 — PROV evaluates business rules and applies change
PROV verifies that the caller has sufficient rights to perform the mutation under their tenant/operator scope. If allowed, PROV applies the mutation and returns the result.

### Step 5 — MDU maps responses or errors
MDU converts the response from PROV into the northbound MDU contract:
- Creation responses return `201 Created` with `EntityDetail` or `VenueDetail`.
- Update responses return `200 OK` with the updated detail schema.
- Deletions return `204 No Content` upon success.
- If PROV returns a validation or state conflict error (e.g., duplicate names), MDU translates it into a normalized MDU error (e.g., `400 Bad Request` or `409 Conflict`).

### Step 6 — MDU returns response
MDU returns the final response payload to the caller.

---

# 7. User Scope Assignments Workflow

This workflow describes how northbound clients manage and query user assignments to operator, entity, or venue scopes.

## Goal

Retrieve, create, or remove access bindings (scope assignments) for users, keeping PROV as the source of truth for all role/policy bindings.

## Step-by-Step Runtime Workflow

### Step 1 — Querying Scoped Assignments
1. A client calls `GET /mdu/users/{userId}/entities` carrying the caller's bearer token.
2. MDU validates the request parameters and token against OWSEC.
3. MDU calls PROV to fetch the target user's management role and policy bindings across all entities and venues.
4. MDU normalizes these bindings into `UserAssignmentsResponse` containing the `items` array of `UserAssignment` objects.
5. MDU returns the list with a `200 OK` status.

### Step 2 — Assigning a User to a Scope
1. A client calls `POST /mdu/users/{userId}/entities` with a `CreateUserAssignmentRequest` body containing `scopeType`, `scopeId`, and `role`.
2. MDU validates the request body and token.
3. MDU forwards the assignment request to PROV. PROV checks if the caller has permissions to delegate access, binds the user to the scope/role, and assigns any management policies.
4. PROV returns the created binding. MDU maps it to a `UserAssignment` object and returns it with a `201 Created` status.

### Step 3 — Removing a Scoped Assignment
1. A client calls `DELETE /mdu/users/{userId}/entities/{entityId}`.
2. MDU validates parameters and forwards the request to PROV to unbind the user from the entity.
3. PROV removes the role/policy assignment.
4. MDU returns a `204 No Content` status.

---

# 8. Timeout and Retry Behavior

Phase 1 downstream calls must use explicit bounded timeouts.

Safe read operations may use limited retries where the retry behavior is approved and does not create incorrect duplicate effects.

Write or mutation operations must not be retried automatically unless the operation is explicitly known to be safe for retry.

If a downstream timeout or retryable dependency failure still prevents correct completion, MDU must return the normalized dependency failure response instead of hiding the failure.

---

# 9. Error Workflow in Phase 1

Every Phase 1 API follows the same error model.

This section is a workflow-level error summary. The full normalized error model for Phase 1 is defined by the Phase 1 specification and OpenAPI.

## Local validation failures

If the request is invalid before downstream execution, MDU returns a normalized local error such as:

- `validation_error`
- `unauthorized`

## OWSEC failures

If token validation fails, MDU returns an auth-related normalized error and stops processing.

## PROV access failures

If PROV denies access or scope, MDU returns the correct normalized Mango-facing access error such as:

- `forbidden`
- `not_found`

## PROV dependency failures

If PROV times out, becomes unavailable, or returns an invalid response, MDU maps that into normalized dependency errors such as:

- `downstream_timeout`
- `downstream_unavailable`
- `bad_gateway`
- `dependency_auth_failed`
- `rate_limited`

## Internal failures

Unexpected failures map to:

- `internal_error`

If a route explicitly allows degraded or incomplete data, that behavior must be documented in the contract. Otherwise MDU should fail rather than return ambiguous mixed-success data.

MDU must not expose raw downstream stack traces or internal secrets.

---

# 10. Implementation Boundary Note

Phase 1 implementation must preserve clean service boundaries:

- handlers remain thin
- authentication, request guards, and correlation handling remain in middleware
- orchestration and response shaping remain in the service layer
- downstream calls remain behind dedicated adapters or clients

This workflow document describes runtime behavior, but it does not replace the implementation structure rules defined by the phase spec and common requirements.

---

# 11. Observability Workflow in Phase 1

Every Phase 1 request should be observable end to end.

## Logging

Each request should carry enough information for supportability:

- request ID
- correlation ID
- route/operation
- downstream dependency touched
- normalized result

## Metrics

Phase 1 should emit metrics for:

- request count
- latency
- status distribution
- downstream dependency latency
- downstream dependency failures

## Tracing

Phase 1 should preserve traceability across:

- inbound MDU request
- OWSEC validation call
- PROV downstream call
- final response path

---

# 12. Runtime Workflow at Startup

Phase 1 startup must fail fast if critical configuration is invalid.

## Startup sequence

1. service loads config
2. validates listener/runtime config
3. validates downstream/auth/internal settings
4. prepares required clients/adapters
5. starts HTTP server only when the service is safe to accept traffic

If critical configuration is missing or unsafe, startup must fail instead of running in a broken partial mode.

---

# 13. What We Get After Phase 1

After Phase 1, MDU will be a real integrated service with:

- Mango-facing authenticated APIs
- OWSEC-backed token validation
- PROV-backed foundational business workflows
- normalized Mango-facing responses
- standardized auth flow
- consistent validation and error handling
- production-ready logging, metrics, tracing, liveness, readiness support where required by platform contract, and CI verification

Phase 1 does **not** yet add Billing, OWGW, Topology, or Analytics workflows.

But it gives the working service foundation that later phases will build on.

---

# 14. Final Phase 1 Workflow Summary

Phase 1 request flow is:

```text
UI
 -> OWSEC login already completed
 -> call MDU with bearer token
 -> MDU validates token through OWSEC
 -> MDU validates request
 -> MDU calls PROV with x-api + x-authorization
 -> PROV resolves access and returns source-of-truth data
 -> MDU normalizes response
 -> UI receives Mango-facing contract
```

That is the core Phase 1 workflow model for MDU Service.

---

# 15. UI View and Functionality to API Mapping

This section maps the UI actions, views, and dashboards in Phase 1 directly to the API endpoints they call (across MDU, OWSEC, and PROV), describing request parameters and response shapes.

## 1. App Initialization & Session Bootstrap
- **Functionality:** Load caller profile, permissions, and active scope on app launch.
- **API Called:** `GET /mdu/me` (MDU)
- **Request:**
  * Headers: `Authorization: Bearer <owsec-token>`
- **Response:** `200 OK` with `SessionContext` (`user`, `activeScope`, `assignments`, `permissions`).
- **Workflow:**
  1. UI sends request with bearer token to MDU.
  2. MDU validates token via OWSEC and retrieves operational context from PROV.
  3. MDU returns consolidated `SessionContext` to the UI to determine available features.

## 2. Organization Tree (Sidebar Navigation)
- **Functionality:** Render the expandable hierarchy tree of entities and venues.
- **API Called:** `GET /mdu/hierarchy/tree` (MDU)
- **Request:**
  * Headers: `Authorization: Bearer <owsec-token>`
  * Query Parameters: `scopeEntityId` (optional UUID)
- **Response:** `200 OK` with `HierarchyTreeResponse` (nested array of roots with `HierarchyNode` and `HierarchyNodeSummary` containing sub-entity/venue/user counts).
- **Workflow:**
  1. UI requests the tree structure under the current scoped entity.
  2. MDU queries PROV for the entity tree, calculates node summary metrics, and returns the nested structure.

## 3. Entity Workspace (Overview and Child Listings)
- **Functionality:** View details of selected entity (site, building) and list its child entities.
- **APIs Called:**
  * View Detail: `GET /mdu/entities/{entityId}` (MDU)
  * List Children: `GET /mdu/entities?parentEntityId={entityId}` (MDU)
- **Request:**
  * Path/Query: `entityId` or `parentEntityId` (UUID)
- **Response:** `EntityDetail` or `EntityListResponse` (array of `EntitySummary` containing `id`, `name`, `parentId`, `type`, ancestry `path`, user/venue/device counts).
- **Workflow:**
  1. UI requests details or child lists for the active entity.
  2. MDU queries PROV downstream, maps fields, and returns them to the UI.

## 4. Venue Workspace (Listings and Details)
- **Functionality:** View venue hierarchy (e.g. floors, rooms) and venue details under an entity.
- **APIs Called:**
  * List Venues: `GET /mdu/entities/{entityId}/venues` (MDU)
  * View Detail: `GET /mdu/venues/{venueId}` (MDU)
- **Request:**
  * Path/Query: `entityId` or `venueId` (UUID), `parentVenueId` (optional query filter)
- **Response:** `VenueListResponse` (array of `VenueSummary`) or `VenueDetail` (adds descriptions, policy/role IDs).
- **Workflow:**
  1. UI requests venues associated with the active entity/parent venue.
  2. MDU retrieves the records from PROV and normalizes the venue detail.

## 5. Team/Users Management & Profile CRUD
- **Functionality:** Retrieve the user list and manage admin-user profiles (CRUD).
- **APIs Called (Direct to OWSEC):**
  * List Users: `GET /users`
  * Create User: `POST /user`
  * Update User: `PUT /user/{id}`
  * Delete User: `DELETE /user/{id}`
- **Request:**
  * Headers: `Authorization: Bearer <owsec-token>`
  * Query Parameters (List): `offset`, `limit`
  * Body (Create/Update): User profile fields
- **Response:** `UserInfoList` or single `UserInfo` object.
- **Workflow:**
  1. Since the user database is owned entirely by OWSEC, user list retrieval and account mutations bypass MDU and call OWSEC directly.
  2. OWSEC validates the caller session and returns or modifies the user record directly.

## 6. User Scope Assignments (Cross-System Mapping)
- **Functionality:** Bind users (from OWSEC) to specific operational entity/venue scopes and management roles (from PROV).
- **APIs Called (via MDU):**
  * List Assignments: `GET /mdu/users/{userId}/entities`
  * Grant Assignment: `POST /mdu/users/{userId}/entities`
  * Revoke Assignment: `DELETE /mdu/users/{userId}/entities/{entityId}`
- **Request:**
  * Body (Grant): `CreateUserAssignmentRequest` (`scopeType`, `scopeId`, `role`)
- **Response:** `UserAssignmentsResponse` (list), `201 Created` with `UserAssignment` (grant), or `204 No Content` (revoke).
- **Workflow:**
  1. To bind a user identity to a resource scope, the UI calls MDU.
  2. MDU acts as the orchestrator: it verifies the user ID exists in OWSEC and requests PROV to record the role/policy binding for that user on the target entity/venue.

## 7. Customers / Operators Management
- **Functionality:** View and manage customer profiles (modeled as operators).
- **APIs Called (Direct to PROV):**
  * List Customers: `GET /operator`
  * View Detail: `GET /operator/{uuid}`
  * Create Customer: `POST /operator`
  * Update Customer: `PUT /operator/{uuid}`
  * Delete Customer: `DELETE /operator/{uuid}`
- **Request:**
  * Query Parameters (List): `offset`, `limit`
  * Request Body (Create/Update): `Operator` fields
- **Response:** `OperatorList` or single `Operator` object.
- **Workflow:**
  1. Since customer/operator database tables reside in PROV, operator list and profile mutations bypass MDU and call PROV directly.
  2. PROV applies standard tenant bounds and updates the operator record.

## 8. Entity & Venue CRUD Modals
- **Functionality:** Create, update, or delete entities and venues.
- **APIs Called:**
  * Create Entity: `POST /mdu/entities` (MDU)
  * Update Entity: `PATCH /mdu/entities/{entityId}` (MDU)
  * Delete Entity: `DELETE /mdu/entities/{entityId}` (MDU)
  * Create Venue: `POST /mdu/entities/{entityId}/venues` (MDU)
  * Update Venue: `PATCH /mdu/venues/{venueId}` (MDU)
  * Delete Venue: `DELETE /mdu/venues/{venueId}` (MDU)
- **Request:**
  * Request Bodies: `CreateEntityRequest`, `UpdateEntityRequest`, `CreateVenueRequest`, or `UpdateVenueRequest`.
- **Response:** `201 Created` with detail, `200 OK` with updated detail, or `204 No Content` on deletion.
- **Workflow:**
  1. UI submits mutations through MDU.
  2. MDU forwards the requests downstream to PROV.
