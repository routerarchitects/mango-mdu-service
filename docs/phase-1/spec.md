# Mango MDU Service — Phase 1 Specification

## Purpose

This document defines the Phase 1 specification for `mango-mdu-service`.

Phase 1 is the foundation phase for MDU Service. Its purpose is to make MDU the Mango-facing authenticated orchestration layer for the first set of operator-domain APIs, with real OWSEC and PROV integration.

This document defines what Phase 1 must include, what it must not include, and what must be delivered for Phase 1 to be complete.

---

## Phase 1 Goal

The goal of Phase 1 is to establish MDU Service as the service that:

- accepts Mango-facing API requests under `/api/v1/*`
- validates OWSEC bearer tokens before protected operations
- calls downstream services using service credentials
- forwards user context to PROV where required
- exposes normalized Mango-facing contracts
- preserves OWSEC as the authoritative owner for user identity and credentials
- preserves PROV as the source of truth for operators, hierarchy, roles, policies, and RBAC

In simple terms, Phase 1 makes MDU the first real backend entry point for Mango operator workflows.

---

## What Phase 1 Includes

Phase 1 includes:

- service bootstrap and production-ready runtime setup.
- authenticated Mango-facing APIs under `/api/v1/*` requiring validated bearer-token authentication through OWSEC.
- inbound bearer-token validation through OWSEC.
- token-backed session/bootstrap view APIs:
  - `GET /api/v1/session`.
- operator APIs and user-access orchestration APIs (assignments, access policies) as approved Phase 1 northbound wrapper contracts over downstream services.
- complete resource management wrapper APIs (entities, venues, roles, policies) delegating state persistence to PROV.
- user-scoped assignment APIs (for user roles and access scopes) and access-policy management.
- subscriber list retrieval for operators.
- service-to-service downstream calls using internal service credentials.
- forwarding user bearer context to PROV using `x-authorization` where required.
- access-summary style workflows where PROV remains the RBAC decision-maker.
- normalized request validation.
- normalized error handling, supporting return of `401`, `403`, `404`, `409`, and `503` responses where appropriate in a stable `ApiError` envelope.
- request tracing, request ID, and correlation ID propagation.
- a clean production route baseline without placeholder scaffold CRUD surfaces.

All operator, entity, venue, role, and policy APIs in Phase 1 act as MDU-facing normalized wrapper contracts, and user endpoints are limited to access orchestration (assignments, access policies) while user identity and CRUD remain directly with OWSEC.

---

## What Phase 1 Does Not Include

Phase 1 does not include:

- MDU-owned login or session issuance.
- local RBAC or policy persistence (PROV is the source of truth).
- local user or operator database storage (OWSEC/PROV are the sources of truth).
- billing integrations (billing is out of scope for Phase 1).
- live device runtime integration.
- topology integration.
- analytics integration.
- async job/workflow durability.
- advanced admin/debug workflow APIs.

These belong to later phases unless the requirements are formally changed.

---

## API Classes and Auth Posture

Phase 1 APIs are primarily **UI-facing authenticated APIs**.

These are the Mango-facing APIs called by frontend or equivalent user-context clients.

Rules for Phase 1:

- production business APIs in this phase shall be treated as UI-facing authenticated APIs
- they must require validated user/session credentials
- they must not rely on frontend hiding for authorization
- they must return normalized MDU-facing responses

Internal-only APIs are not part of the normal Phase 1 product surface unless explicitly introduced and documented.

If any internal-only endpoint is introduced in Phase 1, it:

- must use approved internal authentication
- must not treat a frontend bearer token as sufficient by default
- must not be exposed as a normal Mango-facing business route

Admin/debug/support APIs are out of the normal Phase 1 business scope except minimal operational support endpoints such as health/readiness where required by the platform.

For the current runtime baseline, the operational support surface is:

- `GET /livez` on both public and private ports without authentication (only the public interface on port `16010` is documented in the Phase 1 OpenAPI spec for simplicity).
- `/api/v1/system` on both public and private ports through the shared subsystem/system-routes module.

`/api/v1/system` is not a Mango-facing Phase 1 business API. It is an operational support API with an explicitly documented **multi-mode auth rule**:

- on the public port (port `16010`) it uses validated bearer-token auth (when public authentication is enabled via `AUTH_ENABLED=true`), which is documented in the Phase 1 OpenAPI contract.
- on the private port (port `17010`) it uses the approved internal authentication model (handled by the private interface `AuthHandler` middleware), which is kept internal and omitted from the client-facing OpenAPI spec.

This multi-mode rule is intentional and must remain explicit in repo-tracked API contract and requirements documents so the security posture is not ambiguous.

---

## Downstream Systems Used in Phase 1

Phase 1 actively depends on:

- **OWSEC** for token validation and identity/session-related checks
- **PROV** for operator-domain workflows and RBAC/source-of-truth decisions

Phase 1 does not require active business integration with:

- Billing Service
- OWGW
- NW Topology Service
- OWANALYTICS

---

## Authentication and Authorization Rules

Phase 1 must follow these rules:

- the UI authenticates directly with OWSEC
- protected MDU APIs accept `Authorization: Bearer <owsec-token>`
- protected MDU APIs accept optional request tracking headers:
  - `X-Request-Id`: a unique identifier for the request, generated by the service if absent.
  - `X-Correlation-Id`: a tracking identifier linking requests across distributed system components, set to `X-Request-Id` or generated if absent.
- MDU validates the token before protected business logic (when enabled via the `AUTH_ENABLED` configuration)
- MDU calls downstreams using service credentials such as `x-api`
- MDU forwards the user token to PROV using `x-authorization` where PROV needs user context. MDU propagates tracing headers downstream where applicable; if downstream services do not accept them explicitly, MDU still uses them for internal observability/log correlation.
- PROV remains responsible for RBAC and scope decisions
- MDU must not create a second source of truth for access control

Phase 1 must reject requests when:

- authentication credentials are missing
- the bearer token is invalid
- the caller is outside the allowed authorization scope
- the workflow requires downstream authorization and that authorization is not granted

Authorization in Phase 1 must be enforced through the approved flow and must cover, where relevant:

- resource-level authorization
- scope-level authorization
- action-level authorization
- role/profile restrictions

MDU does not become the RBAC authority in Phase 1, but it must still enforce the protected service boundary correctly.

---

## Phase 1 API Inventory

All Phase 1 MDU business APIs listed below require validated bearer-token authentication (via the `Authorization: Bearer <token>` header, when enabled via the `AUTH_ENABLED` configuration). Requests with missing or invalid credentials must be rejected with a `401 Unauthorized` response when token validation is enabled. Support routes may have different authentication posture (specifically, `/livez` is unauthenticated). Additionally, all routes (with the exception of `/livez`) accept the optional `X-Request-Id` and `X-Correlation-Id` tracking headers to enable tracing across distributed system components.

> [!NOTE]
> For Phase 1, collection-level operator operations (specifically listing all operators and creating a new operator) are handled directly by hitting PROV endpoints (`GET /operator` and `POST /operator/{uuid}`). Due to PROV's downstream schema, the operator creation path is member-style: clients issue a `POST /operator/{uuid}` where `{uuid}` is set to the nil/zero UUID (`00000000-0000-0000-0000-000000000000` or `0`). Only individual operator operations (`GET`, `PUT`, `DELETE` under `/api/v1/operators/{operatorId}`) are routed through MDU. This hybrid routing model is mandatory: standard clients must call PROV directly for list/create operations, and call MDU for detailed operator member operations.

### 1. Session / Access Context (`Session` Tag)
- `GET /api/v1/session` — Retrieve active session and effective access context.

### 2. Operators (`Operators` Tag)
- `GET /api/v1/operators/{operatorId}` — Retrieve operator details.
- `PUT /api/v1/operators/{operatorId}` — Update operator details.
- `DELETE /api/v1/operators/{operatorId}` — Delete operator.

### 3. Subscribers (`Subscribers` Tag)
- `GET /api/v1/operators/{operatorId}/subscribers` — Retrieve a simple, unpaginated list of subscriber signup entries filtered by operator ID (constrained listing flow).

### 4. Hierarchy (`Hierarchy` Tag)
- `GET /api/v1/hierarchy/tree` — Retrieve full or scoped resource hierarchy tree.

### 5. Entities (`Entities` Tag)
- `GET /api/v1/entities` — List entities.
- `POST /api/v1/entities` — Create a new entity.
- `GET /api/v1/entities/{entityId}` — Retrieve details of a specific entity.
- `PUT /api/v1/entities/{entityId}` — Update entity details.
- `DELETE /api/v1/entities/{entityId}` — Delete entity.
- `GET /api/v1/entities/{entityId}/venues` — List venues under an entity.
- `POST /api/v1/entities/{entityId}/venues` — Create a new venue under an entity.

### 6. Venues (`Venues` Tag)
- `GET /api/v1/venues/{venueId}` — Retrieve venue details.
- `PUT /api/v1/venues/{venueId}` — Update venue details.
- `DELETE /api/v1/venues/{venueId}` — Delete venue.

### 7. Management Policies (`Management Policies` Tag)
- `GET /api/v1/policies` — List management policies.
- `POST /api/v1/policies` — Create a new management policy.
- `GET /api/v1/policies/{policyId}` — Retrieve details of a specific policy.
- `PUT /api/v1/policies/{policyId}` — Update management policy details.
- `DELETE /api/v1/policies/{policyId}` — Delete management policy.

### 8. Management Roles (`Management Roles` Tag)
- `GET /api/v1/roles` — List management roles.
- `POST /api/v1/roles` — Create a new management role.
- `GET /api/v1/roles/{roleId}` — Retrieve details of a specific role.
- `PUT /api/v1/roles/{roleId}` — Update management role details.
- `DELETE /api/v1/roles/{roleId}` — Delete management role.

### 9. Users / Scoped Assignments & Access (`Users` Tag)
- `GET /api/v1/users/{userId}/assignments` — List resource assignments for a user.
- `POST /api/v1/users/{userId}/assignments` — Assign resource (entity/venue) scope to a user (handles creation, updating/resolving existing roles, or no-op/idempotent success).
- `DELETE /api/v1/users/{userId}/assignments/{assignmentId}` — Remove a user scope assignment.
- `GET /api/v1/users/{userId}/access-policy` — Get user access policy (requires `scope`, `entityId`, and optional `venueId` query parameters).
- `PUT /api/v1/users/{userId}/access-policy` — Update user access policy.

### 10. Operational Support & Diagnostics (Support Routes)
- `GET /livez` — Liveness/health probe check (unauthenticated; public port `16010` is part of the authoritative Phase 1 OpenAPI contract).
- `GET /api/v1/system` — Retrieve system diagnostics (public port requires `bearerAuth` and is part of the Phase 1 OpenAPI contract).
- `POST /api/v1/system` — Modify diagnostics log levels (public port requires `bearerAuth` and is part of the Phase 1 OpenAPI contract).

---

## MDU-facing Normalized Wrapper Contract and Source of Truth

The operator, entity, venue, role, policy, and user access orchestration APIs are MDU-facing normalized wrapper contracts over downstream services, NOT a transfer of domain ownership or persistent truth. MDU acts as a stateless facade/orchestrator:
- **OWSEC** is the authoritative source of truth for user identity, credentials, login, token validation, and user CRUD. User CRUD does not route through MDU.
- **PROV** is the authoritative source of truth for operators, entities, venues, roles, policies, and persisted RBAC structures. MDU forwards the caller's user context to PROV to validate authorization and retrieve/persist these records.
- **Hybrid Routing:** Collection-level operator operations (listing and creating operators) bypass MDU and are called directly to PROV by standard clients. Individual operator member operations (retrieval, updates, deletion) are routed through the MDU facade.

### Operator and User-Access Lifecycles

Phase 1 provides method and lifecycle coverage for Operator and User-Access API operations as wrapper and orchestration contracts:

- **Operators:** Support member details retrieval (`GET`), updating parameters (`PUT`), and deletion (`DELETE`) through the MDU facade. In alignment with the hybrid routing model, collection-level operator operations (listing and creating operators) bypass MDU and are called directly to PROV by standard clients.
- **User-Access Orchestration:** MDU does not expose full user CRUD. Instead, it supports user scope assignments (`GET` assignments, `POST` assignment additions—which resolve idempotently to create, update, or return success on existing roles—and `DELETE` assignment removals) and access policy lookup (`GET`) and policy updates (`PUT`). User account lifecycle and profile CRUD remain directly with OWSEC.
- **Resource Management (Entities & Venues):** Support full CRUD operations (`GET`, `POST`, `PUT`, `DELETE`) representing MDU wrappers over PROV's hierarchy tree.
- **Access Policies & Roles:** Support listing, creation, reading details, updating, and deleting policies and roles, backed entirely by PROV.

#### Role Distinction

The MDU architecture and API contract distinguish between two different types of roles:
*   **Global Identity Roles (`RoleKey` enum):** These represent the static, system-wide account types defined and enforced by the identity provider (OWSEC) (such as `root`, `admin`, `csr`). These are immutable system classes.
*   **Dynamic Management Roles (`ManagementRole` resource):** These are dynamic, custom role-templates created within Provisioning (PROV) to bind policies, users, and hierarchy nodes together. Because operators can define and name their own resource templates (e.g. "Custom Venue Admin Template"), this resource uses a free-form string for its descriptive name, while the assigned user's base identity role remains bound to the fixed `RoleKey` classification.

**Design Alignment Decisions:**
*   **Assignment and Session Role Modeling:** Although user scope assignments map to dynamic `ManagementRole` templates downstream, the northbound `CreateUserAssignmentRequest`, `UserAssignment`, and `SessionAssignment` schemas model the `role` property using the fixed `RoleKey` enum. This enforces system-wide identity classifications and ensures alignment with OWSEC's security constraints. As a result, Phase 1 user assignments only support this fixed allowlist of global identity roles, and custom PROV management role templates are out of scope for these assignment endpoints.
*   **Management Policy Entry User Bindings Parity & Behavior:** To maintain strict 1:1 parity with the downstream PROV database schema, the northbound `ManagementPolicyEntry` schema retains the `users` array. MDU does not persist this array locally; instead, it acts as a stateless facade and forwards it directly to downstream PROV as-is during policy creation and updates, where PROV persists it in the system of record. Although not ignored or rejected, standard Mango-facing client applications should primarily manage user bindings via the `ManagementRole` and assignments endpoints rather than modifying the policy `users` array directly.


---

## Known PROV Route Families Relevant to Phase 1

The master requirements identify the following current known PROV route families relevant to the MDU foundation baseline:

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
- PROV-owned operator routes as required by the implementation baseline

These are downstream route families MDU is expected to use for Phase 1 foundation work where applicable.

---

## Downstream Call Model for PROV

For Phase 1 PROV-backed workflows, MDU shall call PROV using:

```http
x-api: <mdu-service-api-key>
x-authorization: Bearer <owsec-token>
x-request-id: <request-id>
x-correlation-id: <correlation-id>
```

This means:

- MDU authenticates to PROV as a trusted internal service
- MDU forwards the caller bearer token where PROV needs user context
- PROV resolves RBAC, scope, operator, and user permissions
- MDU shapes the final Mango-facing response

Phase 1 must not bypass PROV authorization by trying to replace it with local logic.

---

## Implementation Rules for Phase 1

Phase 1 must preserve a clean service structure.

Required implementation rules:

- handlers must remain thin
- request parsing and response encoding must stay in the transport layer
- auth, request guards, and correlation handling must stay in middleware
- orchestration and business composition must stay in the application/service layer
- downstream integrations must stay behind explicit adapters or clients
- local persistence, if any, must remain limited to approved MDU-owned operational concerns
- handlers must not contain scattered downstream orchestration logic
- MDU must not accumulate hidden ownership through convenience persistence

---

## OWSEC and PROV Integration Expectations

### OWSEC

For Phase 1, the OWSEC integration must define and implement:

- validation/auth model
- token validation flow
- timeout behavior
- retry behavior where safe
- response normalization into MDU auth semantics
- error translation for auth failures and dependency failures
- traceability/logging for validation calls

### PROV

For Phase 1, the PROV integration must define and implement:

- service-auth model
- `x-authorization` forwarding behavior where required
- timeout behavior
- retry behavior where safe
- response normalization into Mango-facing contracts
- error translation into normalized MDU error responses
- tracing and dependency visibility

Phase 1 does not need endpoint-by-endpoint downstream documentation in this file, but the integration expectations must exist and be implemented.

---

## Validation and Error Handling

Phase 1 must provide:

- validation of required authentication
- validation of path, query, and request-body inputs
- normalized error responses
- consistent handling of downstream failures
- no leakage of raw downstream internal errors to Mango-facing clients

At minimum, the service must support normalized error categories such as:

- `validation_error`
- `unauthorized`
- `forbidden`
- `not_found`
- `conflict`
- `rate_limited`
- `bad_gateway`
- `dependency_auth_failed`
- `downstream_timeout`
- `downstream_unavailable`
- `not_implemented`
- `partial_data`
- `internal_error`

Phase 1 should avoid partial-data responses unless a route explicitly defines them.

If a Phase 1 route allows partial data, the route must define:

- whether the HTTP status stays successful or becomes degraded
- which sections may be missing or partial
- how the dependency failure is indicated

If a route does not define partial-data behavior, it should fail rather than return an ambiguous mixed-success response.

---

## Runtime, Config, and Database Guardrails

Phase 1 must provide:

- Dockerized deployment
- liveness endpoints, and readiness support where required by the platform contract
- structured logging
- request ID and correlation ID support
- traceable downstream calls
- fail-fast startup on missing or invalid critical configuration
- graceful shutdown behavior
- validated runtime configuration for OWSEC, PROV, auth settings, internal credentials, timeouts, and observability

If local database usage exists in Phase 1, it must follow these guardrails:

- only PostgreSQL may be used for approved local MDU-owned state
- no downstream-owned business truth may be stored locally
- no future-phase tables should be introduced early unless Phase 1 actually requires them

---

## Common Modules and Shared Baseline

Phase 1 implementation must use the approved common modules or approved equivalent wrappers for shared infrastructure concerns.

At minimum, the Phase 1 baseline must cover:

- structured logging
- build/version metadata
- service discovery where applicable
- service-to-service HTTP/RPC behavior
- request tracing/correlation propagation
- shared auth/header propagation behavior

Phase 1 should not duplicate common infrastructure logic in individual handlers or downstream clients without an approved reason.

---

## Observability and Operational Requirements

Phase 1 must provide:

- structured logging
- request ID and correlation ID support
- metrics for request count, latency, dependency latency, and dependency failures
- traceable downstream calls
- liveness endpoints, and readiness support where required by the platform contract
- Dockerized deployment
- CI-backed automated verification

For Phase 1 mutation workflows, auditability must be sufficient to identify who called the route, which route ran, and what downstream dependencies were touched.

---

## OpenAPI and Testing Requirements

Every production Phase 1 endpoint must be represented in OpenAPI.

For Phase 1, OpenAPI must include:

- request definitions
- success responses
- error responses
- auth expectations
- explicit multi-mode auth rules where an endpoint is intentionally available through more than one interface/auth class
- examples where appropriate

In accordance with the repository documentation authority:
- `docs/phase-1/mango-mdu-openapi.yaml` is the authoritative contract for Phase 1.
- `docs/openapi.yaml` is the master draft / multi-phase document.
- Right now, the phase-specific OpenAPI spec takes priority for the code in that phase, and the master draft will be aligned once each phase implementation is completed.
- Important APIs should include request/response examples, error examples, auth expectations, and scope notes where useful to maintain contract clarity. Outdated or conflicting documentation must be updated or removed to maintain internal consistency.

For the current runtime and contract baseline, this specifically means:

- `/livez` is documented as unauthenticated on both ports
- `/api/v1/system` is documented as an operational support API, not a Mango-facing business API
- `/api/v1/system` documents both its public bearer-token mode and its private internal-auth mode

Phase 1 testing and CI must cover at minimum:

- formatting checks
- vet/lint/static analysis where configured
- unit tests
- handler/API tests
- service-layer tests
- downstream adapter tests
- auth middleware tests
- OpenAPI/schema validation where applicable
- Docker build
- startup smoke test

“CI-backed automated verification” in Phase 1 means the required checks actually run and pass.

---

## What We Will Have After Phase 1

After Phase 1, we will have:

1. a real Mango-facing MDU API service
2. authenticated APIs under `/api/v1/*`
3. OWSEC-based token validation at the MDU boundary
4. PROV-backed foundational operator, entity, venue, role, policy, and user-access orchestration APIs
5. normalized Mango-facing contracts instead of raw downstream behavior
6. consistent validation and error handling
7. production-ready tracing, logging, metrics, and readiness behavior
8. no placeholder scaffold APIs remaining in the claimed production contract
9. OpenAPI-covered production endpoints
10. CI-verified foundation behavior for auth, handlers, and downstream adapters

In short, after Phase 1 we will have the **foundation MDU service** with real OWSEC and PROV integration, ready for later expansion into billing, device, topology, analytics, and advanced workflow phases.

---

## Phase 1 Completion Criteria

Phase 1 is complete only when:

- protected APIs validate OWSEC bearer tokens and reject unauthenticated requests with `401`
- downstream calls use service auth correctly
- user context is forwarded to PROV where required
- `GET /api/v1/session` is working as the bootstrap API
- all listed Session, Operator, User, Hierarchy, Entity, Venue, Policy, and Role endpoints are available and match the methods defined in the Phase 1 OpenAPI spec
- no placeholder scaffold production routes remain in claimed scope
- normalized validation and error handling are implemented, returning `401`, `403`, `404`, `409`, and `503` responses where appropriate
- logging, tracing, metrics, and readiness are in place
- OpenAPI and implementation are aligned
- config validation and fail-fast startup are implemented
- required automated tests pass in CI
- Docker build and startup smoke test pass
- the operational support routes that remain (`/livez` and `/api/v1/system`) are documented with the same exposure and auth posture implemented in runtime
- no hidden local ownership or placeholder behavior remains in claimed scope

---

## Final Rule

Phase 1 is the **foundation release** of MDU Service.

It makes MDU the authenticated orchestration entry point for Mango operator workflows, with real PROV-backed foundational APIs, while keeping OWSEC as the authoritative owner for users and authentication, and PROV as the system of record for operators, hierarchy, and RBAC truth.
