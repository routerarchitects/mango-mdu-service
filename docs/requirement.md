# Mango MDU Service Master Requirements

## Document Purpose

This document defines the master requirements for the full lifecycle of `mango-mdu-service`.

`mango-mdu-service` is the MangoCloud operator-domain orchestration service for MDU workflows. It exposes versioned Mango-facing APIs under `/api/v1/*`, validates inbound access, shapes UI-facing business contracts, and coordinates calls to downstream systems that own authentication, operators, hierarchy, RBAC, inventory, configuration, billing, live device operations, topology, and analytics.

This document is the whole-service master specification and roadmap baseline. Phase-specific requirements documents shall inherit from this document and from `docs/common-requirement.md`.

This document is intentionally a commit-ready service requirements baseline, not only an architecture note. It defines service boundaries, trust rules, phased scope, workflow expectations, validation direction, and operational behavior while keeping the content lean enough to maintain.

---

## Document Control

| Field | Value |
|---|---|
| Document Title | Mango MDU Service Master Requirements |
| Service Name | `mango-mdu-service` |
| Repository | `routerarchitects/mango-mdu-service` |
| Service Type | Internal orchestration microservice |
| Primary Language | Go |
| Runtime | Dockerized Go service |
| Status | Master baseline |
| Last Updated | 2026-07-02 |
| Primary Consumers | Mango Operator UI and approved internal callers |
| Primary Downstream Services | OWSEC, PROV, Billing Service, OWGW, NW Topology Service, OWANALYTICS |
| Base API Namespace | `/api/v1/*` |
| Common Cross-Phase Rules | `docs/common-requirement.md` |

---

# 1. Executive Summary

MDU Service shall be the Mango-facing orchestration layer for the MDU product domain.

The browser/UI authenticates directly with OWSEC. OWSEC owns login, session issuance, browser bearer tokens, and token validation primitives. After login, the UI calls MDU business APIs using the OWSEC-issued bearer token.

MDU validates inbound requests, normalizes business-facing contracts, composes multi-service payloads, forwards approved user context to downstream private APIs, hides downstream route quirks (except where direct hybrid routing is explicitly approved), and returns versioned UI-facing responses. MDU shall not become a second source of truth for domains already owned by downstream services.

OWSEC is the authoritative owner for user identity (user CRUD, login, token validation, and token issuance). MDU owns only user-access orchestration (assignments, access policies, and role/policy/scope mapping around users) in Phase 1; full user CRUD is handled directly between the UI and OWSEC. 



The downstream trust model is:

1. MDU authenticates to downstream services using service-to-service credentials such as `x-api` or equivalent.
2. MDU forwards the end-user bearer token to downstream services using `x-authorization` when the downstream service requires user context.
3. The downstream service, especially PROV, interprets that forwarded user context and enforces its own authorization and RBAC.
4. MDU does not resolve PROV RBAC locally and does not persist RBAC truth.

OWSEC remains the system of record/authoritative owner for user accounts. PROV remains the system of record for operators, hierarchy, roles, policies, inventory ownership, and configuration ownership. Billing Service remains the system of record for billing. OWGW remains the system of record for live runtime operations. NW Topology Service remains the system of record for topology computation. OWANALYTICS remains the system of record for telemetry and historical analytics. MDU owns orchestration, shaping, composition, and approved workflow state only. Standard clients call PROV directly for operator list (`GET /operator`) and create (`POST /operator/{uuid}` where `{uuid}` is set to the nil/zero UUID `00000000-0000-0000-0000-000000000000` or `0`) operations (bypassing MDU), while retrieving and updating detailed operator members are routed through the MDU facade `/api/v1/operators/{operatorId}`. This hybrid routing model is deliberate and approved.

---

# 2. Core Architecture Rules

1. MDU shall expose Mango-facing APIs under `/api/v1/*`.
2. The UI shall authenticate directly with OWSEC for user identity and credentials.
3. Protected MDU business APIs shall accept `Authorization: Bearer <owsec-token>`.
4. MDU shall validate the inbound bearer token using OWSEC-owned validation mechanisms before executing protected business APIs.
5. MDU shall call downstream private APIs using trusted service credentials such as `x-api` or equivalent.
6. MDU shall forward the inbound user token to downstream private APIs using `x-authorization` when the downstream service needs user context.
7. PROV shall resolve user, operator, scope, role, policy, and RBAC decisions using its own source-of-truth data.
8. MDU shall not become a separate RBAC, hierarchy, operator, user, inventory, configuration, billing, topology, or analytics source of truth.
9. MDU shall normalize downstream responses and errors into versioned UI-facing contracts.
10. MDU shall hide downstream route quirks and compatibility paths from the UI (except for approved documented hybrid routing exceptions, such as operator list/create).
11. Local persistence in MDU shall be minimal and limited to MDU-owned operational concerns.
12. MDU shall remain an orchestration service, not a replacement for downstream domain services.
13. Stability within `/api/v1/*` shall be additive by default; breaking contract changes require an approved versioning or migration plan.
14. Token validation behavior may be online or cached per the approved OWSEC integration contract, but protected APIs shall enforce token validity, expiry, and required caller claims before orchestration.
15. API family names in this document describe workflow groupings, not a requirement to mirror downstream resource trees one-for-one.

---

# 3. Normative Document Set and Authority

This master document is normative for service scope, ownership boundaries, trust lanes, and the phase roadmap.

`docs/common-requirement.md` is normative for cross-phase engineering rules, security guardrails, CI expectations, runtime requirements, observability requirements, and documentation synchronization rules.

Repo-tracked phase documents are normative only for the phase they define.

In accordance with the repository documentation authority:
- `docs/phase-1/mango-mdu-openapi.yaml` is the authoritative contract for Phase 1.
- `docs/openapi.yaml` is the master draft / multi-phase document.
- Right now, the phase-specific OpenAPI spec takes priority for the code in that phase, and the master draft will be aligned once each phase implementation is completed.
- Important APIs should include request/response examples, error examples, auth expectations, and scope notes where useful to maintain contract clarity. Outdated or conflicting documentation must be updated or removed to maintain internal consistency.

---

# 4. Canonical Domain Vocabulary

MDU shall use the following business-facing terms unless a phase-specific contract explicitly narrows or extends them.

| Term | Meaning | Ownership |
|---|---|---|
| operator | top-level Mango tenant or operator scope | PROV |
| entity | hierarchy object used for ownership and tree structure | PROV |
| venue | location or venue object within the hierarchy | PROV |
| workspace | Mango UI-facing composed context for a selected node or tenant | MDU composed view |
| hierarchy node | normalized tree node exposed by MDU | PROV truth, MDU shape |
| user | operator-facing account represented through PROV and secured by OWSEC | OWSEC (identity/account CRUD), PROV (access/rbac bindings only) |
| access summary | composed view of effective access visible to UI | PROV truth, MDU shape |
| device | managed inventory device | PROV truth, OWGW runtime |
| configuration | assignable or viewable config object | PROV |
| subscription | plan or billing relationship | Billing Service |
| billing summary | composed billing-facing view for UI | Billing Service truth, MDU shape |
| topology node | graph or computed topology object | NW Topology Service |
| analytics summary | KPI or historical analytics payload | OWANALYTICS |

Rules:

1. Business-facing terms shall be preferred over raw downstream route names.
2. Raw downstream naming may remain in internal adapters and approved debug/admin APIs.
3. MDU shall not rename source-of-truth identifiers unless it also preserves a stable mapping rule.

---

# 5. Trust Lanes and Security Model

Header names shown in this document are canonical examples only. HTTP header matching remains case-insensitive, but implementation and OpenAPI shall use one canonical spelling consistently per header.

## 5.1 Browser-to-OWSEC Auth Lane

The browser/UI uses OWSEC for login and session issuance. OWSEC owns the public authentication boundary.

```text
UI -> OWSEC
OWSEC -> UI: bearer token
```

## 5.2 Browser-to-MDU Business Lane

The browser/UI calls MDU business APIs using the OWSEC-issued bearer token.

```http
Authorization: Bearer <owsec-token>
```

MDU validates the token with OWSEC before business orchestration. Validation shall cover token validity, expiry, and caller identity claims required by the target workflow. If validation results are cached, the cache TTL shall remain bounded by the approved OWSEC integration contract and shall not outlive token expiry.

## 5.3 MDU-to-Downstream Private Lane

MDU calls downstream services as a trusted internal service and forwards user context separately when required.

```http
x-api: <mdu-service-api-key>
x-authorization: Bearer <owsec-token>
x-request-id: <request-id>
x-correlation-id: <correlation-id>
```

The downstream service decides how to use `x-authorization`. For PROV, RBAC, scope, user, and operator checks are resolved inside PROV.

## 5.4 MDU-to-Billing Private Lane

MDU calls Billing Service as a trusted internal service after PROV resolves the caller's RBAC and scope for the target billing workflow. Billing Service does not receive the browser bearer token directly for the billing calls covered by this document.

```http
X-Internal-API-Key: <mdu-service-api-key>
X-Actor-Id: <userId>
X-Actor-Type: <actor-type>
X-Actor-Role: <actor-role>
X-Tenant-Id: <operatorId-or-tenantId>
X-Request-Id: <request-id>
X-Correlation-Id: <correlation-id>
X-Source-Service: mango-mdu-service
```

Billing Service is the current exception to the standard downstream forwarding rule. MDU shall authenticate using the trusted internal service credential, shall propagate actor and tenant audit context, and shall not forward the end-user bearer token unless an approved integration contract explicitly changes the billing contract.

## 5.5 Security Rules

1. OWSEC owns login, token issuance, and token/session validation primitives.
2. MDU validates inbound protected requests before orchestration.
3. MDU authenticates to downstreams using service credentials.
4. MDU forwards user context only where needed by downstream authorization or business logic.
5. Billing-service calls shall use service credentials plus required actor and tenant headers unless a later approved billing integration contract explicitly changes that model.
6. Downstream systems retain their own authorization rules.
7. MDU shall not bypass downstream authorization by using only machine credentials for user-sensitive workflows.
8. MDU shall not persist browser bearer tokens.

---

# 6. Ownership Boundaries

| Domain / Workflow | System of Record | MDU Role |
|---|---|---|
| Login and token issuance | OWSEC | Not owned by MDU (direct UI-to-OWSEC interaction) |
| Token validation | OWSEC | MDU consumes validation |
| Users / Identity / CRUD | OWSEC | Not owned by MDU (handled directly between UI and OWSEC) |
| User access orchestration | MDU / PROV | MDU orchestrates user assignments and access-policies |
| Operators | PROV | MDU exposes operator-backed workflow APIs |
| Entities / hierarchy | PROV | MDU exposes normalized hierarchy APIs |
| Venues | PROV | MDU exposes normalized venue APIs |
| Roles and policies | PROV | MDU exposes access-management role/policy orchestration |
| RBAC and scope resolution | PROV | MDU does not resolve or persist RBAC |
| Inventory ownership | PROV | MDU wraps and composes inventory APIs |
| Configuration ownership | PROV | MDU wraps and composes configuration APIs |
| Billing plans/subscriptions | Billing Service | MDU composes billing summaries |
| Live device runtime/actions | OWGW | MDU exposes approved action wrappers |
| Topology graph computation | NW Topology Service | MDU composes topology views |
| Telemetry and historical analytics | OWANALYTICS | MDU composes analytics views |
| UI-facing orchestration | MDU | MDU owns API shaping and workflow composition |
| Workflow/job state where approved | MDU | MDU owns only approved operational state |

Rules:

1. MDU owns API shaping, request validation, response composition, error normalization, and approved orchestration logic.
2. MDU does not own persistent truth for downstream-owned domains.
3. MDU may maintain minimal operational state only where explicitly approved by phase scope.
4. Approved local operational state may include request correlation metadata, async operation state, idempotency records, audit support data, reconciliation markers, and bounded caches.
5. MDU shall not persist authoritative business truth for users, operators, hierarchy, policies, roles, inventory, configuration, billing, topology, or analytics.

---

# 7. Repository and Implementation Baseline

The current `mango-mdu-service` repository provides the implementation skeleton. Scaffold behavior and sample item CRUD are not domain truth.

The implementation shall preserve this structure:

- `cmd/` for startup
- `internal/app/` for application wiring
- `internal/http/` for routing, middleware, and transport concerns
- `internal/services/` for orchestration and use cases
- `internal/models/` for normalized API and domain DTOs
- `external/` for downstream clients
- `internal/db/` only for MDU-owned persistence concerns

Rules:

1. Production route groups shall remove or isolate placeholder scaffold APIs.
2. Downstream-specific code shall remain in adapters/clients, not leak into business-facing handlers.
3. Local DB usage shall be confined to MDU-owned operational concerns.

---

# 8. Downstream Dependency Model

The route families in this section are current known downstream inputs for planning and integration design. They are not the authoritative contract by themselves; authoritative behavior must remain tied to repo-tracked requirements, design, and OpenAPI artifacts. Inclusion here means the dependency is known to the service roadmap, not that every listed route family is active in every phase.

## 8.1 OWSEC

OWSEC owns authentication and session security.

MDU shall use OWSEC for:

- validating inbound bearer tokens
- validating API keys where required
- resolving token/session validity
- retrieving caller profile where explicitly needed

Current known OWSEC routes include:

- `POST /oauth2` for login/session issuance, called directly by the UI
- `DELETE /oauth2/{token}` for logout/session removal where applicable
- `GET /validateToken?token=...` for token validation
- `GET /validateSubToken?token=...` where sub-token validation is required
- `GET /validateApiKey?apikey=...` where API-key validation is required
- `GET /oauth2?me=true` if current caller profile is required from OWSEC

## 8.2 PROV

OWSEC is the authoritative owner for user identity, login, token validation, token issuance, and user CRUD. PROV owns operators, entities, venues, roles, policies, RBAC, hierarchy, inventory ownership, and configuration ownership.

MDU shall call PROV using service authentication and forwarded user context:

```http
x-api: <mdu-service-api-key>
x-authorization: Bearer <owsec-token>
```

PROV is responsible for resolving RBAC, scope, operator, and user permissions.

Current known PROV route families include:

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


## 8.3 Billing Service

Billing Service owns plans, subscriptions, billing lifecycle, invoices, billing periods, usage estimates, and billing state. MDU exposes the Mango-facing billing workflows, orchestrates RBAC validation through PROV for every UI-facing billing call, and then calls Billing Service using the approved internal billing contract. MDU may compose billing views and workflow responses but shall not persist billing truth.

For user-facing billing workflows, the normal sequence is:

1. UI calls MDU with `Authorization: Bearer <owsec-token>`.
2. MDU validates the token with OWSEC.
3. MDU calls PROV with service auth plus `x-authorization` to resolve RBAC and scope for the target operator, entity, or billing workflow.
4. If PROV authorizes the action, MDU calls Billing Service with `X-Internal-API-Key` plus actor, tenant, and trace headers.
5. Billing Service returns Billing-owned data; MDU shapes the final Mango-facing response.

MDU shall remain the only Mango-facing entry point for billing workflows. UI clients and PROV shall not call Billing Service directly for the business workflows covered by MDU contracts.

Current known Billing Service route families include:

- `/api/v1/billing/plans`
- `/api/v1/billing/accounts`
- `/api/v1/billing/accounts/{tenant_id}`
- `/api/v1/billing/accounts/{tenant_id}/assigned-plans`
- `/api/v1/billing/accounts/{tenant_id}/assigned-plans/{plan_code}`
- `/api/v1/billing/accounts/{tenant_id}/subscription/plan`
- `/api/v1/billing/accounts/{tenant_id}/subscription`
- `/api/v1/billing/accounts/{tenant_id}/subscription/cancel`
- `/api/v1/billing/accounts/{tenant_id}/invoices`
- `/api/v1/billing/accounts/{tenant_id}/invoices/{invoice_id}`
- `/api/v1/billing/accounts/{tenant_id}/invoices/{invoice_id}/download`
- `/api/v1/billing/accounts/{tenant_id}/billing-period`
- `/api/v1/billing/accounts/{tenant_id}/current-usage`

MDU shall use these Billing Service APIs for the following workflow groups as they become part of approved MDU phase scope:

- static plan discovery and visibility management
- billing account creation and retrieval
- subscription creation or ensure
- subscription plan change
- subscription cancellation
- subscription retrieval
- invoice listing, invoice details, and invoice download proxying
- billing-period lookup
- current-usage and guest-portal summary retrieval

Rules:

1. PROV remains responsible for RBAC, scope, hierarchy, and user permission checks for every UI-facing billing call.
2. Billing Service remains responsible for billing state transitions, invoice truth, plan truth, subscription truth, usage values, and billing-period truth.
3. MDU shall not forward the user bearer token to Billing Service for these workflows.
4. MDU shall propagate actor, tenant, and trace metadata required by the Billing Service contract.
5. MDU shall not expose Billing Service directly as a public API surface; MDU owns the Mango-facing billing API contracts and orchestration only.
6. MDU shall not persist authoritative billing truth locally.

## 8.4 OWGW

OWGW owns live device runtime operations, command execution, and diagnostics. MDU shall expose only approved and validated action wrappers rather than raw unfiltered command access.

MDU shall call OWGW using service authentication and only for approved operational workflows.

Current known OWGW route families include:

- `/device/{serialNumber}`
- `/device/{serialNumber}/logs`
- `/device/{serialNumber}/healthchecks`
- `/device/{serialNumber}/capabilities`
- `/device/{serialNumber}/statistics`
- `/device/{serialNumber}/status`
- `/device/{serialNumber}/configure`
- `/device/{serialNumber}/upgrade`
- `/device/{serialNumber}/reboot`
- `/device/{serialNumber}/trace`
- `/device/{serialNumber}/telemetry`
- selected device action routes required by approved MDU workflows only

OWGW-backed MDU work shall focus on device runtime reads, diagnostics, and approved actions only.


## 8.5 NW Topology Service

NW Topology Service owns topology graph computation. MDU shall consume topology outputs and shape them into workspace-friendly responses.

MDU shall call Topology Service using the approved downstream auth model for topology reads.

Current known Topology Service route families include:

- `/livez`
- `/topology`
- `/system`

The current known topology read shape centers on `GET /topology` with `boardId` and optional `interval` query parameters. Once a topology contract is committed in this repo, that committed artifact shall be the authoritative source for topology request and response details.


## 8.6 OWANALYTICS

OWANALYTICS owns telemetry, historical timepoints, client history, and analytics data. MDU shall consume analytics outputs for summaries, trends, health views, and dashboards.

MDU shall call OWANALYTICS using service authentication for approved read-only analytics workflows.

Current known OWANALYTICS route families include:

- `/board/{id}/devices`
- `/board/{id}/timepoints`
- `/wifiClientHistory`
- `/wifiClientHistory/{client}`



---

# 9. Request Context and Header Propagation Rules

## 9.1 Inbound Headers

Protected business APIs shall accept or generate the following context:

- `Authorization: Bearer <owsec-token>` for end-user auth
- `X-Request-Id` when supplied by caller or gateway
- `X-Correlation-Id` when supplied by caller or gateway
- additional approved tracing headers where supported

## 9.2 Generated Context

If `X-Request-Id` is absent, MDU shall generate one.

If `X-Correlation-Id` is absent, MDU shall use `X-Request-Id` as the correlation ID or generate both when needed.

## 9.3 Outbound Headers to Downstreams

MDU shall propagate:

- `x-api: <service credential>` or equivalent
- `x-authorization: Bearer <owsec-token>` when user context is required by downstream
- `x-request-id`
- `x-correlation-id`
- `X-Actor-Id`, `X-Actor-Type`, optional `X-Actor-Role`, and `X-Tenant-Id` for Billing Service workflows
- `X-Source-Service` for downstream auditability where required by contract
- approved trace headers where supported

## 9.4 Propagation Rules

1. `Authorization` is the inbound UI-facing auth header.
2. `x-authorization` is the downstream private user-context forwarding header.
3. `x-api` or equivalent is the machine-to-machine auth header.
4. Downstream service credentials and end-user tokens serve different purposes and shall not be conflated.
5. MDU shall not log raw token or secret values.
6. MDU shall preserve traceability across all orchestrated downstream calls.
7. For billing workflows, MDU shall first obtain the authorization decision from PROV and then propagate actor context, tenant context, request ID, correlation ID, and required source-service metadata to Billing Service.
8. Implementation and OpenAPI shall choose a single canonical spelling for each header and use it consistently across the service contract.

---

# 10. Validation, Error Handling, and NFR Rules

## 10.1 Validation Rules

1. Path parameters shall be validated for presence and basic format.
2. Query parameters shall be validated for type, bounds, and allowed enumerations where defined.
3. Request bodies shall be validated for required fields, field types, and illegal combinations before downstream orchestration.
4. Unsupported enum values shall return a validation error.
5. MDU shall validate allow-listed action names for OWGW-backed action routes.
6. MDU shall not accept write requests that implicitly create local truth for downstream-owned domains.
7. For list APIs such as entities, venues, management policies, and management roles, MDU shall support standard offset-based pagination query parameters (limit and offset) and resource-specific filtering parameters matching the capabilities of the downstream service.

## 10.2 Error Rules

MDU shall expose a consistent error envelope for UI-facing APIs.

Normalized error categories shall include at minimum:

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

Rules:

1. MDU shall not expose raw downstream stack traces, secrets, or internal implementation details.
2. MDU shall map downstream failures into versioned UI-facing semantics.
3. Pure write APIs shall not return partial success unless the workflow is explicitly modeled to do so.
4. Composed read APIs may return partial data only if the route contract explicitly allows it.
5. Downstream authentication failures, invalid downstream responses, and dependency throttling shall not collapse into a generic internal error when a more precise normalized category applies.
6. All protected UI-facing Phase 1 endpoints require validated bearer-token authentication through OWSEC.
7. Protected endpoints must reject missing or invalid credentials (returning `401 Unauthorized` responses), not just unauthorized scopes (which return `403 Forbidden`).
8. In alignment with the authoritative Phase 1 OpenAPI spec, protected routes must explicitly document and support the return of relevant error codes (such as `401`, `403`, `404`, `409`, and `503` where applicable) in a stable `ApiError` envelope.


## 10.3 Retry, Timeout, and Failure-Isolation Rules

1. MDU shall remain a mostly stateless orchestration service. When billing mutation APIs are introduced, MDU shall support idempotent retry behavior according to the approved route contract and may persist only the minimal idempotency or workflow state required for safe retries.
2. MDU shall not automatically retry write or action calls unless the downstream operation is explicitly safe to retry.
3. MDU may use limited retries for safe read calls when failures are transient.
4. Every downstream call shall use a bounded timeout, and MDU shall return normalized timeout or dependency-failure errors.
5. Composed read APIs shall use bounded fan-out and shall not expand hierarchy, venue, device, or child-node data without limits in a single request.
6. If an optional read dependency such as Billing, Analytics, or Topology is unavailable, MDU may return partial data only where the route contract explicitly allows it.
7. Write APIs shall fail cleanly and shall not return partial success unless the workflow is explicitly designed for it.

## 10.4 Partial-Data Contract Rules

If a route allows partial data, the route contract shall define:

1. whether the HTTP status remains success or becomes a degraded-success response
2. which response sections may be absent, null, empty, or explicitly marked degraded
3. how the response indicates which dependency failed
4. whether retry guidance is exposed to the caller
5. whether partial results are cacheable

Routes that do not define partial-data behavior shall fail rather than return an ambiguous mixed-success payload.

## 10.5 Device Action Safety Rules

1. Device action APIs shall use an approved allow-list of supported actions.
2. MDU shall not expose raw command passthrough or unrestricted runtime command execution.
3. Device action routes shall define clear timeout, validation, and normalized error-mapping behavior.

## 10.6 Operational and NFR Rules

1. MDU shall be observable, traceable, and safe to operate as a dependency-orchestration service.
2. MDU shall expose health and readiness endpoints according to platform conventions.
3. Structured logs shall include request ID, correlation ID, route or operation name, downstream service name where applicable, and normalized result code.

---

# 11. Development Phases

The phases are cumulative. Each phase shall preserve ownership boundaries and security rules from previous phases.

The phase sections describe roadmap scope only. They do not override the cross-phase requirements in `docs/common-requirement.md`, and the listed API families are indicative workflow groupings rather than a mandatory one-to-one route inventory.

## Phase 1: Orchestrator Foundation

### Objective

Establish the service foundation, auth boundary, downstream trust model, and core PROV/OWSEC integration.

### Scope

Phase 1 includes:

- service bootstrap and configuration.
- inbound OWSEC bearer-token validation.
- direct OWSEC login boundary.
- service-authenticated downstream calls.
- `x-authorization` forwarding to downstream services.
- token-backed session/bootstrap view APIs (where MDU acts as a northbound wrapper, not a login authority).
- operator wrapper APIs and user-access orchestration APIs (assignments, access policies) as approved Phase 1 northbound wrapper contracts over downstream services.
- complete resource management wrapper APIs (entities, venues, roles, policies) delegating state persistence to PROV.
- user-scoped assignment APIs (for user roles and access scopes) and access-policy management.
- subscriber list retrieval for operators.
- access-summary workflows where PROV provides the RBAC decision.
- normalized errors (401, 403, 404, 409, 503) and observability.
- removal of production placeholder routes.

All operator, entity, venue, role, and policy APIs in Phase 1 act as MDU-facing normalized wrapper contracts, and user endpoints are limited to access orchestration (assignments, access policies) while user identity and CRUD remain directly with OWSEC.

### Non-Goals

Phase 1 does not require:

- MDU-owned login/signup or session issuance.
- local RBAC or policy persistence (PROV is the source of truth).
- local user or operator database storage (OWSEC/PROV are the sources of truth).
- billing workspace aggregation or billing integrations (billing is out of scope for Phase 1).
- live device aggregation or live device status.
- topology aggregation.
- analytics dashboards.
- async workflow persistence.

### Phase 1 API Inventory

All Phase 1 MDU business APIs listed below require validated bearer-token authentication (via the `Authorization: Bearer <token>` header, when enabled via the `AUTH_ENABLED` configuration). Requests with missing or invalid credentials must be rejected with a `401 Unauthorized` response when token validation is enabled. Support routes may have different authentication posture (specifically, `/livez` is unauthenticated). Additionally, all routes (with the exception of `/livez`) accept the optional `X-Request-Id` and `X-Correlation-Id` tracking headers to enable tracing across distributed system components.

> **Operator Routing Strategy:**
> For Phase 1, collection-level operator operations (specifically listing all operators and creating a new operator) are handled directly by hitting PROV endpoints (`GET /operator` and `POST /operator/{uuid}`). Due to PROV's downstream schema, the operator creation path is member-style: clients issue a `POST /operator/{uuid}` where `{uuid}` is set to the nil/zero UUID (`00000000-0000-0000-0000-000000000000` or `0`). Only individual operator operations (`GET`, `PUT`, `DELETE` under `/api/v1/operators/{operatorId}`) are routed through MDU. This hybrid routing model is mandatory: standard clients must call PROV directly for list/create operations, and call MDU for detailed operator member operations.

#### 1. Session / Access Context (`Session` Tag)
- `GET /api/v1/session` — Retrieve active session and effective access context.

#### 2. Operators (`Operators` Tag)
- `GET /api/v1/operators/{operatorId}` — Retrieve operator details.
- `PUT /api/v1/operators/{operatorId}` — Update operator details.
- `DELETE /api/v1/operators/{operatorId}` — Delete operator.

#### 3. Subscribers (`Subscribers` Tag)
- `GET /api/v1/operators/{operatorId}/subscribers` — Retrieve a simple, unpaginated list of subscriber signup entries filtered by operator ID (constrained listing flow).

#### 4. Hierarchy (`Hierarchy` Tag)
- `GET /api/v1/hierarchy/tree` — Retrieve full or scoped resource hierarchy tree.

#### 5. Entities (`Entities` Tag)
- `GET /api/v1/entities` — List entities.
- `POST /api/v1/entities` — Create a new entity.
- `GET /api/v1/entities/{entityId}` — Retrieve details of a specific entity.
- `PUT /api/v1/entities/{entityId}` — Update entity details.
- `DELETE /api/v1/entities/{entityId}` — Delete entity.
- `GET /api/v1/entities/{entityId}/venues` — List venues under an entity.
- `POST /api/v1/entities/{entityId}/venues` — Create a new venue under an entity.

#### 6. Venues (`Venues` Tag)
- `GET /api/v1/venues/{venueId}` — Retrieve venue details.
- `PUT /api/v1/venues/{venueId}` — Update venue details.
- `DELETE /api/v1/venues/{venueId}` — Delete venue.

#### 7. Management Policies (`Management Policies` Tag)
- `GET /api/v1/policies` — List management policies.
- `POST /api/v1/policies` — Create a new management policy.
- `GET /api/v1/policies/{policyId}` — Retrieve details of a specific policy.
- `PUT /api/v1/policies/{policyId}` — Update management policy details.
- `DELETE /api/v1/policies/{policyId}` — Delete management policy.

#### 8. Management Roles (`Management Roles` Tag)
- `GET /api/v1/roles` — List management roles.
- `POST /api/v1/roles` — Create a new management role.
- `GET /api/v1/roles/{roleId}` — Retrieve details of a specific role.
- `PUT /api/v1/roles/{roleId}` — Update management role details.
- `DELETE /api/v1/roles/{roleId}` — Delete management role.

#### 9. Users / Scoped Assignments & Access (`Users` Tag)
- `GET /api/v1/users/{userId}/assignments` — List resource assignments for a user.
- `POST /api/v1/users/{userId}/assignments` — Assign resource (entity/venue) scope to a user (handles creation, updating/resolving existing roles, or no-op/idempotent success).
- `DELETE /api/v1/users/{userId}/assignments/{assignmentId}` — Remove a user scope assignment.
- `GET /api/v1/users/{userId}/access-policy` — Get user access policy (requires `scope`, `entityId`, and optional `venueId` query parameters).
- `PUT /api/v1/users/{userId}/access-policy` — Update user access policy.

#### 10. Operational Support & Diagnostics (Support Routes)
The MDU operational support surface is registered and exposed on both public and private port interfaces:

*   **`GET /livez` (Liveness Check):** Registered on both public and private ports without authentication. (Only the public version on port `16010` is documented in the Phase 1 OpenAPI contract for simplicity).
*   **`/api/v1/system` (Diagnostics):** Registered on both public and private ports with a multi-mode authentication rule:
    *   On the public port (port `16010`), it uses validated bearer-token auth (`bearerAuth`, when enabled via the `AUTH_ENABLED` configuration), which is formally documented in the Phase 1 OpenAPI contract.
    *   On the private port (port `17010`), it uses the approved internal authentication model (handled by the private interface `AuthHandler` middleware), which is kept internal and omitted from the client-facing OpenAPI spec.

### Completion Criteria

Phase 1 is complete only when:

1. protected business APIs validate OWSEC bearer tokens and reject unauthenticated requests with `401` (when authentication is enabled via `AUTH_ENABLED=true`).
2. downstream private calls use service auth plus forwarded user context where required.
3. scaffold placeholder APIs are removed or isolated from production routes.
4. all listed Session, Operator, User, Hierarchy, Entity, Venue, Policy, and Role endpoints are available and match the methods defined in the Phase 1 OpenAPI spec.
5. normalized error handling (401, 403, 404, 409, 503) and structured correlation are implemented.
6. source-of-truth ownership rules remain preserved.

## Phase 2: Billing and Workspace Dashboard Integration

### Objective

Make MDU useful for operator workspace tab integrations, billing administration, and hierarchy browsing/lazy-loading.

### Scope

Phase 2 includes:

- operator workspace overview and tab dashboard payload aggregation.
- hierarchy browsing, lazy child node expansion, and deep tree searches.
- billing account management, subscription creation, plan updates, billing period retrieval, invoices, usage reports, and invoice download proxies backed by Billing Service.

### Non-Goals

Phase 2 does not require:

- live device actions.
- topology graph APIs.
- analytics dashboards.
- durable workflow engine.
- broad async jobs.

### Indicative API Families

- `/api/v1/hierarchy/*` (lazy node expansion)
- `/api/v1/workspaces/*`
- `/api/v1/billing/plans/*`
- `/api/v1/billing/accounts/*`
- `/api/v1/billing/subscriptions/*`
- `/api/v1/billing/invoices/*`
- `/api/v1/bootstrap/*` where approved

---

## Phase 3: Devices and Configurations

### Objective

Add operational device and configuration workflows by composing PROV inventory/configuration truth with OWGW live runtime data.

### Scope

Phase 3 includes:

- device inventory wrappers backed by PROV
- venue-device assignment backed by PROV
- configuration CRUD and assignment backed by PROV
- live status and diagnostics backed by OWGW
- approved device action wrappers backed by OWGW

### Non-Goals

Phase 3 does not require:

- full topology workspaces
- historical analytics dashboards
- rollout engine
- AI hooks

### Indicative API Families

- `/api/v1/devices/*`
- `/api/v1/configurations/*`
- `/api/v1/venues/{venueId}/devices`
- `/api/v1/devices/{serialNumber}/actions/*`

## Phase 4: Topology, Analytics, Metrics, and Health

### Objective

Add cross-domain operational visibility using topology and analytics as first-class downstream dependencies.

### Scope

Phase 4 includes:

- topology workspace APIs backed by NW Topology Service
- analytics summaries backed by OWANALYTICS
- KPI and health panels
- map/overlay payloads
- workspace overview aggregation

### Indicative API Families

- `/api/v1/topology/*`
- `/api/v1/analytics/*`
- `/api/v1/maps/*`
- `/api/v1/metrics/*`
- `/api/v1/dashboard/*`

## Phase 5: Workflow Durability and Advanced Operations

### Objective

Introduce durable workflow safety and advanced operational maturity for long-running, multi-system, or recovery-sensitive workflows.

### Scope

Phase 5 includes:

- idempotency keys for selected write APIs
- async jobs and operation status
- workflow tracking where needed
- audit and reconciliation support
- selective forward recovery or compensation
- rollout orchestration hooks
- AI recommendation hooks where approved
- advanced admin/debug APIs

### Indicative API Families

- `/api/v1/operations/*`
- `/api/v1/jobs/*`
- `/api/v1/reconciliation/*`
- `/api/v1/rollouts/*`
- `/api/v1/ai/*`
- `/api/v1/admin/debug/*`
- `/api/v1/audit/*`

---

# 12. Final Architectural Rules

1. `mango-mdu-service` is the Mango operator-domain orchestration service.
2. The UI authenticates directly with OWSEC.
3. OWSEC owns login, session issuance, bearer token validity, and token validation primitives.
4. The UI calls MDU with `Authorization: Bearer <owsec-token>`.
5. MDU validates the inbound token before protected business workflows.
6. MDU calls downstream services with service authentication such as `x-api`.
7. MDU forwards user context to downstream services using `x-authorization` where required.
8. OWSEC is the authoritative owner for user identity, login, token validation, token issuance, and user CRUD. PROV owns operators, hierarchy, policies, roles, RBAC, inventory ownership, and configuration ownership.
9. Billing Service owns billing truth, while MDU owns the Mango-facing billing API contracts and orchestration path.
10. OWGW owns live device runtime and command execution.
11. NW Topology Service owns topology graph computation.
12. OWANALYTICS owns telemetry and historical analytics.
13. MDU owns orchestration, request validation, API shaping, response composition, error normalization, and future approved workflow state.
14. MDU shall not become a second source of truth for downstream-owned domains.
