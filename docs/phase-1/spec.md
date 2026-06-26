# Mango MDU Service — Phase 1 Specification

## Purpose

This document defines the Phase 1 specification for `mango-mdu-service`.

Phase 1 is the foundation phase for MDU Service. Its purpose is to make MDU the Mango-facing authenticated orchestration layer for the first set of operator-domain APIs, with real OWSEC and PROV integration.

This document defines what Phase 1 must include, what it must not include, and what must be delivered for Phase 1 to be complete.

---

## Phase 1 Goal

The goal of Phase 1 is to establish MDU Service as the service that:

- accepts Mango-facing API requests under `/api/v1/mdu/*`
- validates OWSEC bearer tokens before protected operations
- calls downstream services using service credentials
- forwards user context to PROV where required
- exposes normalized Mango-facing contracts
- preserves OWSEC as the authoritative owner for users
- preserves PROV as the source of truth for customers, hierarchy, roles, policies, and RBAC

In simple terms, Phase 1 makes MDU the first real backend entry point for Mango operator workflows.

---

## What Phase 1 Includes

Phase 1 includes:

- service bootstrap and production-ready runtime setup
- authenticated Mango-facing APIs under `/api/v1/mdu/*`
- inbound bearer-token validation through OWSEC
- token-backed bootstrap APIs:
  - `GET /api/v1/mdu/me`
  - `GET /api/v1/mdu/session`
- service-to-service downstream calls using internal service credentials
- forwarding user bearer context to PROV using `x-authorization` where required
- foundational PROV-backed APIs and workflows for:
  - operators
  - entities
  - venues
  - roles
  - policies
  - customers where needed for the foundation baseline
- access-summary style workflows where PROV remains the RBAC decision-maker
- normalized request validation
- normalized error handling
- request tracing, request ID, and correlation ID propagation
- a clean production route baseline without placeholder scaffold CRUD surfaces

The non-business support surface is limited to `/livez` and `/api/v1/system` while Mango-facing `/api/v1/mdu/*` business APIs remain Phase 1 implementation work.

---

## What Phase 1 Does Not Include

Phase 1 does not include:

- MDU-owned login
- MDU-owned session issuance
- local RBAC ownership
- local user or customer source-of-truth ownership
- billing integration
- live device runtime integration
- topology integration
- analytics integration
- async job/workflow durability
- advanced admin/debug workflow APIs

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

- `GET /livez` on both public and private ports without authentication
- `/api/v1/system` on both public and private ports through the shared subsystem/system-routes module

`/api/v1/system` is not a Mango-facing Phase 1 business API. It is an operational support API with an explicitly documented **multi-mode auth rule**:

- on the public port it uses validated bearer-token auth
- on the private port it uses the approved internal authentication model

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
- MDU validates the token before protected business logic
- MDU calls downstreams using service credentials such as `x-api`
- MDU forwards the user token to PROV using `x-authorization` where PROV needs user context
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

## Main Phase 1 APIs

The main Phase 1 API families are:

- `GET /api/v1/mdu/me` — OWSEC is the authoritative owner for user identity; MDU calls PROV to fetch the authenticated user's Mango bootstrap context (operator scope, customer scope, roles, hierarchy visibility) and composes the normalized `/me` response
- `GET /api/v1/mdu/session`
- `/api/v1/mdu/operators/*`
- `/api/v1/mdu/entities/*`
- `/api/v1/mdu/venues/*`
- `/api/v1/mdu/policies/*`
- `/api/v1/mdu/roles/*`
- `/api/v1/mdu/customers/*`

These are Mango-facing APIs. They do not need to mirror downstream APIs exactly, but they must be backed by the approved Phase 1 auth flow and PROV integration where the domain belongs to PROV.

The Phase 1 bootstrap APIs are view/bootstrap endpoints only. They do not mean MDU owns login or session issuance.

> **Note on `GET /api/v1/mdu/me`:** OWSEC owns user CRUD, login, token issuance, and user account identity. PROV provides the logged-in user's operational context for Mango — operator scope, customer scope, roles, policies, hierarchy visibility, and dashboard bootstrap data. MDU composes the final `/me` response from the OWSEC-validated identity plus the PROV-fetched Mango context.

---

## PROV-backed Phase 1 Functionality

Phase 1 is not just a thin API shell. It must use PROV for the foundational business workflows listed in scope.

### Operators

Phase 1 operator APIs shall use PROV operator data and authorization decisions.

### Entities

Phase 1 entity APIs shall use PROV entity and hierarchy data.

Where tree or scope structure is needed, the PROV hierarchy model remains authoritative.

### Venues

Phase 1 venue APIs shall use PROV venue data.

### Policies

Phase 1 policy APIs shall use PROV-owned policy truth.

### Roles

Phase 1 role APIs shall use PROV-owned role truth.

### Customers

Phase 1 customer APIs may be included where needed for the foundation baseline, but they remain PROV-backed and must not imply Billing integration.

### Access Summary

If Phase 1 exposes access-summary or effective-access style APIs, those APIs shall use PROV as the RBAC decision-maker and MDU as the response-shaping layer.

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
- PROV-owned customer routes as required by the implementation baseline

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
- PROV resolves RBAC, scope, customer, and user permissions
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
2. authenticated APIs under `/api/v1/mdu/*`
3. OWSEC-based token validation at the MDU boundary
4. PROV-backed foundational operator-domain APIs
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

- protected APIs validate OWSEC bearer tokens
- downstream calls use service auth correctly
- user context is forwarded to PROV where required
- `GET /api/v1/mdu/me` and `GET /api/v1/mdu/session` are working as bootstrap APIs
- foundational PROV-backed APIs are available
- no placeholder scaffold production routes remain in claimed scope
- normalized validation and error handling are implemented
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

It makes MDU the authenticated orchestration entry point for Mango operator workflows, with real PROV-backed foundational APIs, while keeping OWSEC as the authoritative owner for users and authentication, and PROV as the system of record for customers, hierarchy, and RBAC truth.
