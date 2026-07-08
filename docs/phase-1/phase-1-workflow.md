# Mango MDU Service — Phase 1 Workflow

## Purpose

This document describes how Phase 1 of `mango-mdu-service` works at the workflow level.

It is based on the Phase 1 scope from the MDU master requirements and Phase 1 specification:

- MDU is the Mango-facing authenticated orchestration layer
- MDU does not own login, session issuance, or users (OWSEC is the authoritative owner for identity and user CRUD); operators, hierarchy, roles, policies, and RBAC truth remain with PROV, while MDU orchestrates user-access assignments and access-policies.
- MDU normalizes Mango-facing contracts and hides downstream quirks (except where direct hybrid routing is explicitly approved)

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

Phase 1 includes the following Mango-facing API families:

- `GET /api/v1/session` — OWSEC is the authoritative owner for user identity; MDU calls PROV to fetch the authenticated user's Mango bootstrap context (operator scope, roles, hierarchy visibility) and composes the normalized `/session` response.
- `/api/v1/operators/*` — Standard APIs for managing operators.
- `/api/v1/entities/*` — APIs for entity hierarchy management wrapper.
- `/api/v1/venues/*` — APIs for venue management wrapper.
- `/api/v1/policies/*` — APIs for management policies wrapper.
- `/api/v1/roles/*` — APIs for management roles wrapper.
- `/api/v1/users/*` — User assignments and access-policy orchestration (e.g. `GET /api/v1/users/{userId}/assignments` and `GET /api/v1/users/{userId}/access-policy`).

### Excluded APIs
Contacts, subscriber management (invite/creation), and subscriber devices are not exposed as active MDU contract routes in Phase 1 (with the sole exception of `/api/v1/operators/{operatorId}/subscribers` for operator-scoped listing). They are managed downstream in PROV.

These API families are indicative workflow groupings for Phase 1. They are not a strict one-to-one route inventory and do not require MDU to mirror downstream route structure exactly.

In the current runtime baseline, the checked-in support routes are exposed on both public and private port interfaces:

- **`GET /livez` (Liveness Check):** Registered on both public and private ports without authentication (only the public version on port `16010` is documented in the Phase 1 OpenAPI spec for simplicity).
- **`/api/v1/system` (Diagnostics):** Registered on both public and private ports with a multi-mode authentication rule (token auth for public port `16010` when enabled via `AUTH_ENABLED=true`, and approved internal authentication model handled by the private interface `AuthHandler` middleware for private port `17010`).

Those support routes are outside the Mango-facing business workflow families described in this document. The `/api/v1/*` sections below describe the intended Phase 1 business workflow model to be implemented.

---

## Known PROV Route Families Used in Phase 1

The master requirements identify the following current known PROV route families relevant to the Phase 1 foundation baseline:

- `/operator`
- `/operator/{uuid}`
- `/entity`
- `/entity/{uuid}`
- `/entity?setTree=true`
- `/venue`
- `/venue/{uuid}`
- `/managementPolicy`
- `/managementPolicy/{uuid}`
- `/managementRole`
- `/managementRole/{id}`
- `/subscriber` (for operator-scoped subscriber lists)

Contacts and subscriber device APIs are not active in MDU and remain purely downstream/supporting context in PROV. MDU uses these as downstream sources where the Phase 1 Mango-facing workflow requires them.

---

## Common Headers in Phase 1

### Inbound to MDU

All protected Phase 1 API operations accept optional tracing headers as part of their request contract:

```http
Authorization: Bearer <owsec-token>
X-Request-Id: <request-id>         optional
X-Correlation-Id: <correlation-id> optional
```

If request or correlation IDs are missing, MDU generates them and preserves them across the entire request life-cycle.

### Outbound from MDU to PROV

For user-context PROV workflows, MDU sends:

```http
x-api: <mdu-service-api-key>
x-authorization: Bearer <owsec-token>
x-request-id: <request-id>
x-correlation-id: <correlation-id>
```

This is the standard Phase 1 downstream trust flow. MDU propagates tracing headers downstream where applicable. If downstream services do not accept them explicitly, MDU still uses them for internal observability/log correlation.

---

# 1. Common Phase 1 Request Workflow

Every protected Phase 1 business API follows the same base workflow.

## Step 1 — UI calls MDU

The Mango UI or equivalent caller sends a request to an MDU Phase 1 endpoint under `/api/v1/*`.

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

Before protected business execution, MDU validates the bearer token using OWSEC-owned validation behavior (when token authentication is enabled via the `AUTH_ENABLED` configuration parameter).

Validation confirms at minimum:

- token is present
- token is valid
- token is not expired
- caller identity is usable for the target workflow

If validation fails, MDU returns a normalized auth failure and does not continue to PROV.

## Step 4 — MDU determines target workflow

Once the token is valid, MDU determines which Phase 1 workflow is being executed, such as:

- bootstrap view
- operator lookup
- hierarchy/entity view
- venue view
- role/policy view
- operator details/update/delete workflow
- access-summary style view

## Step 5 — MDU calls PROV where domain truth is required

If the workflow depends on PROV-owned data or authorization, MDU calls PROV with:

- service authentication (`x-api`)
- forwarded user context (`x-authorization`)
- trace headers (`x-request-id`, `x-correlation-id`)

## Step 6 — PROV evaluates access and returns truth

PROV remains responsible for:

- scope decisions
- RBAC decisions
- operator access
- hierarchy visibility
- source-of-truth data for operators, entities, venues, roles, and policies

OWSEC is the authoritative owner for user identity (user CRUD, login, token issuance). PROV provides the user's Mango operational context: operator scope, roles, policies, and hierarchy visibility.

## Step 7 — MDU normalizes response

MDU maps the downstream response into a Mango-facing contract.

This is where MDU adds value:

- normalizes field shape
- hides downstream quirks (except for approved exceptions such as operator list/create)
- maps errors into stable MDU categories
- returns a clean MDU response model

## Step 8 — MDU returns response

The caller receives a normalized Phase 1 response.

---

# 2. Bootstrap Workflow — `GET /api/v1/session`

## Goal

Return the normalized Mango-facing caller context for the authenticated user, composed from OWSEC-validated identity and PROV-fetched operational context.

## Ownership Model

- **OWSEC** owns user CRUD, login, token issuance, and user account identity. The bearer token from OWSEC is the source of the caller's identity claim.
- **PROV** provides the logged-in user's operational context for Mango: operator scope, roles, policies, hierarchy visibility, and dashboard bootstrap data.
- **MDU** composes the final `/session` response from the OWSEC-validated identity plus the PROV-fetched Mango context.

## Workflow

1. UI calls `GET /api/v1/session`
2. MDU validates the bearer token through OWSEC (when enabled via `AUTH_ENABLED=true`) — this confirms user identity
3. MDU calls PROV with service auth plus forwarded user token to fetch the caller's Mango operational context (operator scope, roles, policies, hierarchy visibility)
4. MDU composes the normalized `/session` response from the confirmed OWSEC identity and the PROV-returned context
5. MDU returns the Mango-facing result

## Important Rule

This endpoint is a **view/bootstrap endpoint** only.

It does **not**:

- create login state
- issue tokens
- own session lifecycle

OWSEC is the authoritative owner for user identity. PROV is the source of the user's Mango operational context. MDU does not persist either.

---

# 3. Read Workflow — PROV-backed List or Detail APIs

This is the common flow for Phase 1 read endpoints such as:

- operators
- entities
- venues
- policies
- roles

## Example Shape

The exact MDU route may vary, but the read workflow is the same.

## Workflow

1. UI calls an MDU read endpoint
2. MDU validates authentication through OWSEC
3. MDU validates request parameters
4. MDU decides which PROV route family is needed
5. MDU calls PROV with:
   - `x-api`
   - `x-authorization`
   - request/correlation IDs
6. PROV evaluates caller access and fetches domain truth
7. PROV returns the result or an access failure
8. MDU maps the response into its Mango-facing contract
9. MDU returns the final response

## Examples of downstream mapping

Examples include:

- `/api/v1/operators/*` using PROV `/operator` routes
- `/api/v1/entities/*` using PROV `/entity` routes
- `/api/v1/venues/*` using PROV `/venue` routes
- `/api/v1/policies/*` using PROV `/managementPolicy` routes
- `/api/v1/roles/*` using PROV `/managementRole` routes

MDU may reshape or rename fields, but PROV remains the source of truth.

---

# 4. Hierarchy Workflow — Entities and Tree-Oriented Reads

Phase 1 includes foundational hierarchy-related workflows backed by PROV.

## Goal

Allow MDU to expose Mango-facing hierarchy/entity views while keeping PROV as the hierarchy owner.

## Workflow

1. UI calls an entity or hierarchy-related MDU endpoint
2. MDU validates the token through OWSEC
3. MDU validates path/query inputs
4. MDU calls the appropriate PROV entity route
5. If tree structure is needed, MDU may use the PROV tree-capable entity route family such as `entity?setTree=true`
6. PROV applies scope and visibility rules
7. PROV returns authoritative hierarchy/entity data
8. MDU shapes the hierarchy response for Mango-facing use
9. MDU returns the normalized result

## Important Rule

Phase 1 does **not** create a local hierarchy source of truth inside MDU.

---

# 5. Authorization-sensitive Read Workflow

Some Phase 1 APIs may look like simple reads, but they are still authorization-sensitive because the caller should only see:

- allowed operators
- allowed entities
- allowed venues
- allowed policies/roles
- allowed hierarchy scope

## Workflow

1. caller requests a resource or list
2. MDU validates token via OWSEC
3. MDU forwards user context to PROV
4. PROV decides whether the caller has:
   - resource-level access
   - scope-level access
   - action-level access
   - role/profile-based entitlement
5. MDU returns:
   - normalized success response if authorized
   - normalized forbidden/not-found style response if not authorized

MDU must not rely on frontend hiding to enforce these rules.

---

# 6. Optional Phase 1 Mutation Workflow — if approved for selected PROV-backed domains

If a selected Phase 1 API includes an approved mutation on a PROV-owned domain object, MDU still acts only as the orchestration layer and not as the source of truth.

This section describes the mutation flow only for approved Phase 1 cases. It must not be read as blanket confirmation that all create/update/delete flows are part of Phase 1.

## Workflow

1. UI sends mutation request to MDU
2. MDU validates:
   - bearer token
   - request body
   - path parameters
   - basic business input rules that can be checked locally
3. MDU determines which PROV-owned domain is being changed
4. MDU forwards the request to PROV using:
   - `x-api`
   - `x-authorization`
   - request/correlation IDs
5. PROV evaluates permissions and business rules
6. PROV applies the mutation if allowed
7. PROV returns success or a domain/business/access failure
8. MDU converts the result into a normalized Mango-facing response
9. MDU returns the final response

## Important Rules

- MDU must not store authoritative local truth for the mutated object
- MDU must not bypass PROV permission checks
- MDU must normalize failures instead of leaking raw downstream errors

---

# 7. Access Summary Workflow

Phase 1 may include access-summary style endpoints where the UI needs a summarized view of effective access.

## Goal

Expose a Mango-facing view of access-related information without making MDU the RBAC engine.

## Workflow

1. UI calls the access-summary style endpoint
2. MDU validates the token
3. MDU forwards user context to PROV
4. PROV evaluates current scope and RBAC truth
5. PROV returns the authoritative access facts
6. MDU transforms that result into an MDU-facing summary response
7. MDU returns the normalized response

## Important Rule

PROV remains the RBAC decision-maker.

MDU only shapes the summary.

---

# 8. User, Operator, and Policy Management Workflow

This section describes how northbound clients manage user access, roles, policies, and operator-scoped subscribers.

## Goal
Manage user-related access context, roles, and access-policies in Phase 1, using OWSEC as the source of truth for user accounts and identity, and PROV as the source of truth for operators, roles, policies, hierarchy, and subscribers.

## Architectural Boundaries & Service Division
In Phase 1, all user-related entities belong to one of two services:
1. **OWSEC (User identity, credentials, token authority, and user CRUD):** 
   - Authoritative for basic user profiles (name, email, password, login status, MFA) and user CRUD operations.
   - Serves as the authentication and authorization token authority.
2. **PROV (User roles, policies, and operator data):**
   - **Management Roles & Policies:** Defines the specific permissions and resource scopes (entities, venues) linked to user UUIDs.
   - **Operators:** Manages operator metadata and settings.

MDU acts as the orchestration layer for access-policies and assignments. Contacts, subscriber invite/registration, and subscriber devices are out of scope for Phase 1.

## Detailed Workflows

### 1. User Identity Management (OWSEC)
User identity, credentials, login, session token validation/issuance, and user account CRUD are out of MDU scope and are handled directly between the UI/clients and OWSEC. MDU does not expose user CRUD endpoints.
* **OWSEC Endpoints (Bypassing MDU):**
  * `GET /users` (List users)
  * `GET /user/{id}` (Get user profile)
  * `POST /user` (Create user account)
  * `PUT /user/{id}` (Update user account profile or status)
  * `DELETE /user/{id}` (Delete user account)
* **Orchestration Flow:** These requests bypass MDU entirely and go directly to OWSEC. MDU has no CRUD user database and does not coordinate user account lifecycle events in Phase 1.

### 2. User Roles & Access Policies Management (PROV via MDU)
To retrieve, grant, modify, or revoke scoped resource-level access permissions for a user:

#### Role Distinction note
The MDU API contract distinguishes between two different types of roles:
*   **Global Identity Roles (`RoleKey` enum):** These represent the static, system-wide account types defined and enforced by the identity provider (OWSEC) (such as `root`, `admin`, `csr`). These are immutable system classes.
*   **Dynamic Management Roles (`ManagementRole` resource):** These are dynamic, custom role-templates created within Provisioning (PROV) to bind policies, users, and hierarchy nodes together. Because operators can define and name their own resource templates (e.g. "Custom Venue Admin Template"), this resource uses a free-form string for its descriptive name. However, northbound user scope assignments (`CreateUserAssignmentRequest`, `UserAssignment`, and `SessionAssignment` schemas) only support the fixed `RoleKey` allowlist of global identity roles, meaning custom PROV templates themselves are out of scope for these assignment endpoints.
* **MDU Northbound API Endpoints:**
  * `GET /api/v1/users/{userId}/access-policy?scope={scope}&entityId={entityId}&venueId={venueId}`
  * `PUT /api/v1/users/{userId}/access-policy`
* **Downstream PROV API Endpoints:**
  * **Management Roles:** `GET /managementRole`, `POST /managementRole`, `PUT /managementRole/{id}`, `DELETE /managementRole/{id}`
  * **Management Policies:** `GET /managementPolicy`, `POST /managementPolicy`, `PUT /managementPolicy/{uuid}`, `DELETE /managementPolicy/{uuid}`
* **Orchestration Flow:**
  * **Retrieve Access Policy (GET):**
    1. The client calls `GET /api/v1/users/{userId}/access-policy` specifying `scope` (always required), `entityId` (always required), and `venueId` (required only for venue scope).
    2. MDU validates the authorization token via OWSEC.
    3. MDU calls PROV `GET /managementRole` filtering by `entity` ID (and `venue` ID if requesting venue scope) to locate the role template associated with the user.
    4. MDU calls PROV `GET /managementPolicy/{uuid}` using the policy ID linked to the role to fetch its detailed permission entries.
    5. MDU maps and merges this data into a unified `UserAccessPolicy` payload: if scope is `entity`, only `entityId` is included; if scope is `venue`, both `entityId` and `venueId` are included.
  * **Assign or Update Access Policy (PUT):**
    1. The client submits a `PUT /api/v1/users/{userId}/access-policy` payload detailing the scope (entity/venue), role template, target `entityId` (always required), `venueId` (required only if scope is `venue`), and resource permissions.
    2. MDU validates the request body and authorizes the caller.
    3. MDU interacts with PROV downstream:
       - Creates or updates a `ManagementPolicy` containing entries for the specified resource permissions (assigning resource UUIDs/patterns and access lists).
       - Creates or updates a `ManagementRole` for the target scope (entity or venue), linking it to the `ManagementPolicy`, and ensuring the target user's UUID is in the `users` list.
       > [!NOTE]
       > **ManagementPolicyEntry User Bindings Behavior:** For compatibility with the downstream PROV database schema, `ManagementPolicyEntry` exposes a `users` array. MDU does not persist this array locally; instead, it acts as a stateless facade and forwards it directly to downstream PROV as-is during policy creation and updates, where PROV persists it in the system of record. Although not ignored or rejected, standard Mango-facing client applications should primarily manage user bindings via the `ManagementRole` and assignments endpoints rather than modifying the policy `users` array directly.

    4. MDU returns the normalized `UserAccessPolicy` configuration back to the client.

### 2a. User Scope Assignments (PROV via MDU)
To list, assign, or remove user bindings to entity and venue scopes:
* **MDU Northbound API Endpoints:**
  * `GET /api/v1/users/{userId}/assignments` (Get effective scope assignments)
  * `POST /api/v1/users/{userId}/assignments` (Assign user to entity or venue scope)
  * `DELETE /api/v1/users/{userId}/assignments/{assignmentId}` (Remove assignment by ID)
* **Orchestration Flow:**
  * **List Assignments:** MDU calls PROV `GET /managementRole`, retrieves all roles containing the target `userId`, and maps the associated `entity` or `venue` field to list entries.
  * **Assign User (POST):** 
    - The client sends a `CreateUserAssignmentRequest` containing `scopeType` (`entity` or `venue`), `scopeId`, and `role`.
    - MDU calls PROV `GET /managementRole` to check if a matching role already exists for that scope.
    - MDU resolves the target assignment using the following rules:
      1. **Create (returns 201 Created)**: If no matching scoped role exists, MDU creates one using `POST /managementRole` with the user's UUID in the `users` list.
      2. **Update/Resolve (returns 201 Created)**: If a matching scoped role exists but the target user is not yet bound to it, MDU updates it using `PUT /managementRole/{id}` to add the user's UUID to the `users` array.
      3. **No-Op/Idempotent Success (returns 200 OK)**: If the target user's UUID is already present in the existing matching role's `users` array, MDU returns success immediately without performing any downstream modifications.
      4. **Conflict (returns 409 Conflict)**: If the request conflicts with downstream state in a way that cannot be safely/deterministically resolved (e.g. unresolvable name/ID duplicates), MDU rejects the operation.
  * **Remove Assignment (DELETE):**
    - MDU retrieves the `ManagementRole` identified by `assignmentId` using `GET /managementRole/{id}`.
    - MDU removes the user's UUID from the `users` array and updates the role using `PUT /managementRole/{id}`.
    - If the user was the only member in the role, MDU may optionally delete the role entirely using `DELETE /managementRole/{id}`.

### 3. Excluded Contact Workflows
Contacts and Operator Contacts (backed by PROV `/contact` and `/operatorContact` routes) are out of scope for MDU in Phase 1 and are not exposed as active MDU endpoints.

### 4. Operator Management (PROV via MDU & Direct)
To manage operator profile details (retrieval, updates, and deletion through the MDU facade, or list and create operations directly to PROV):
* **MDU Northbound API Endpoints:**
  * **Operator Paths:**
    * `GET /api/v1/operators/{operatorId}` (Retrieve operator details)
    * `PUT /api/v1/operators/{operatorId}` (Update operator details)
    * `DELETE /api/v1/operators/{operatorId}` (Delete an operator)
* **Direct UI/Client API Endpoints (Bypassing MDU):**
  * `GET /operator` (List all operators in PROV)
  * `POST /operator/{uuid}` (Create a new operator in PROV; `{uuid}` must be set to `00000000-0000-0000-0000-000000000000` or `0` for new creation)
* **Orchestration Flow:**
  * **Hybrid Routing Model:** Standard client applications (e.g. the MDU UI) call PROV directly to list operators (`GET /operator`) and create a new operator (`POST /operator/{uuid}` where `{uuid}` is set to the nil/zero UUID `00000000-0000-0000-0000-000000000000` or `0`). Detail-level operations such as retrieving details, updating name/description, or deleting an operator are routed through the MDU facade. The MDU facade routes enforce OWSEC bearer authentication, while direct PROV list/create operations follow their own approved authentication paths.

### 4a. Management Policies & Roles (PROV via MDU)
To retrieve, create, update, or delete management policies and roles:
* **MDU Northbound API Endpoints:**
  * **Policies:**
    * `GET /api/v1/policies` (List management policies)
    * `POST /api/v1/policies` (Create management policy)
    * `GET /api/v1/policies/{policyId}` (Get policy details)
    * `PUT /api/v1/policies/{policyId}` (Update policy)
    * `DELETE /api/v1/policies/{policyId}` (Delete policy)
  * **Roles:**
    * `GET /api/v1/roles` (List management roles)
    * `POST /api/v1/roles` (Create management role)
    * `GET /api/v1/roles/{roleId}` (Get role details)
    * `PUT /api/v1/roles/{roleId}` (Update role)
    * `DELETE /api/v1/roles/{roleId}` (Delete role)
* **Downstream PROV API Endpoints:**
  * `GET /managementPolicy`, `POST /managementPolicy`, `PUT /managementPolicy/{uuid}`, `DELETE /managementPolicy/{uuid}`
  * `GET /managementRole`, `POST /managementRole`, `PUT /managementRole/{id}`, `DELETE /managementRole/{id}`
* **Orchestration Flow:**
  * **Façade Pattern:** MDU acts as a stateless pass-through for policies and roles, forwarding the requests directly to the downstream PROV endpoints, translating schemas where necessary.

### 5. Operator-scoped Subscriber Management (PROV via MDU)
To list subscriber accounts associated with a specific operator:
* **MDU Northbound API Endpoints:**
  * `GET /api/v1/operators/{operatorId}/subscribers` (List subscribers)
* **Downstream PROV API Endpoints:**
  * `GET /subscriber`
* **Orchestration Flow:**
  * **List Subscribers (Constrained):** MDU calls PROV `GET /subscriber?listOnly=true` to retrieve all signup entries, filters the results to only include those matching the target `operatorId` parameter, and returns the filtered subset as a simple, unpaginated list.
---

# 9. Timeout and Retry Behavior

Phase 1 downstream calls must use explicit bounded timeouts.

Safe read operations may use limited retries where the retry behavior is approved and does not create incorrect duplicate effects.

Write or mutation operations must not be retried automatically unless the operation is explicitly known to be safe for retry.

If a downstream timeout or retryable dependency failure still prevents correct completion, MDU must return the normalized dependency failure response instead of hiding the failure.

---

# 10. Error Workflow in Phase 1

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

# 11. Implementation Boundary Note

Phase 1 implementation must preserve clean service boundaries:

- handlers remain thin
- authentication, request guards, and correlation handling remain in middleware
- orchestration and response shaping remain in the service layer
- downstream calls remain behind dedicated adapters or clients

This workflow document describes runtime behavior, but it does not replace the implementation structure rules defined by the phase spec and common requirements.

---

# 12. Observability Workflow in Phase 1

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

# 13. Runtime Workflow at Startup

Phase 1 startup must fail fast if critical configuration is invalid.

## Startup sequence

1. service loads config
2. validates listener/runtime config
3. validates downstream/auth/internal settings
4. prepares required clients/adapters
5. starts HTTP server only when the service is safe to accept traffic

If critical configuration is missing or unsafe, startup must fail instead of running in a broken partial mode.

---

# 14. What We Get After Phase 1

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

# 15. Final Phase 1 Workflow Summary

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
