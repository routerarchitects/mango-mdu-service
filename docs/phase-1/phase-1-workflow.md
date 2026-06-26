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

Phase 1 includes the following Mango-facing API families:

- `GET /api/v1/mdu/me` — OWSEC is the authoritative owner for user identity; MDU calls PROV to fetch the authenticated user's Mango bootstrap context (operator scope, customer scope, roles, hierarchy visibility) and composes the normalized `/me` response
- `GET /api/v1/mdu/session`
- `/api/v1/mdu/operators/*`
- `/api/v1/mdu/entities/*`
- `/api/v1/mdu/venues/*`
- `/api/v1/mdu/policies/*`
- `/api/v1/mdu/roles/*`
- `/api/v1/mdu/customers/*`

These API families are indicative workflow groupings for Phase 1. They are not a strict one-to-one route inventory and do not require MDU to mirror downstream route structure exactly.

In the current runtime baseline, the checked-in support routes are still limited to:

- `GET /livez`
- `/api/v1/system`

Those support routes are outside the Mango-facing business workflow families described in this document. The `/api/v1/mdu/*` sections below describe the intended Phase 1 business workflow model to be implemented.

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
- PROV-owned customer routes as required by the implementation baseline

MDU uses these as downstream sources where the Phase 1 Mango-facing workflow requires them.

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

Once the token is valid, MDU determines which Phase 1 workflow is being executed, such as:

- bootstrap view
- operator lookup
- hierarchy/entity view
- venue view
- role/policy view
- customer workflow
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
- customer access
- hierarchy visibility
- source-of-truth data for operators, entities, venues, roles, policies, and customers

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

# 2. Bootstrap Workflow — `GET /api/v1/mdu/me`

## Goal

Return the normalized Mango-facing caller context for the authenticated user, composed from OWSEC-validated identity and PROV-fetched operational context.

## Ownership Model

- **OWSEC** owns user CRUD, login, token issuance, and user account identity. The bearer token from OWSEC is the source of the caller's identity claim.
- **PROV** provides the logged-in user's operational context for Mango: operator scope, customer scope, roles, policies, hierarchy visibility, and dashboard bootstrap data.
- **MDU** composes the final `/me` response from the OWSEC-validated identity plus the PROV-fetched Mango context.

## Workflow

1. UI calls `GET /api/v1/mdu/me`
2. MDU validates the bearer token through OWSEC — this confirms user identity
3. MDU calls PROV with service auth plus forwarded user token to fetch the caller's Mango operational context (operator scope, customer scope, roles, policies, hierarchy visibility)
4. MDU composes the normalized `/me` response from the confirmed OWSEC identity and the PROV-returned context
5. MDU returns the Mango-facing result

## Important Rule

This endpoint is a **view/bootstrap endpoint** only.

It does **not**:

- create login state
- issue tokens
- own session lifecycle

OWSEC is the authoritative owner for user identity. PROV is the source of the user's Mango operational context. MDU does not persist either.

---

# 3. Bootstrap Workflow — `GET /api/v1/mdu/session`

## Goal

Return the token-backed bootstrap/session view for the authenticated caller.

## Workflow

1. UI calls `GET /api/v1/mdu/session`
2. MDU validates the bearer token through OWSEC
3. MDU determines the caller bootstrap/session context
4. If required, MDU uses PROV-backed domain context to complete the response
5. MDU returns a normalized session/bootstrap payload

## Important Rule

This endpoint does **not** mean MDU owns browser session issuance.

It is only a Mango-facing bootstrap/session-context view built on top of authenticated user context.

---

# 4. Read Workflow — PROV-backed List or Detail APIs

This is the common flow for Phase 1 read endpoints such as:

- operators
- entities
- venues
- customers
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

- `/api/v1/mdu/operators/*` using PROV `/operator` routes
- `/api/v1/mdu/entities/*` using PROV `/entity` routes
- `/api/v1/mdu/venues/*` using PROV `/venue` routes
- `/api/v1/mdu/policies/*` using PROV `/managementPolicy` routes
- `/api/v1/mdu/roles/*` using PROV `/managementRole` routes

MDU may reshape or rename fields, but PROV remains the source of truth.

---

# 5. Hierarchy Workflow — Entities and Tree-Oriented Reads

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

# 6. Authorization-sensitive Read Workflow

Some Phase 1 APIs may look like simple reads, but they are still authorization-sensitive because the caller should only see:

- allowed operators
- allowed entities
- allowed venues
- allowed customers
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

# 7. Optional Phase 1 Mutation Workflow — if approved for selected PROV-backed domains

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

# 8. Access Summary Workflow

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
