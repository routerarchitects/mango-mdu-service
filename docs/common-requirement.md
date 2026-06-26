# Mango MDU Service Common Requirements

## Purpose

This document defines the implementation, delivery, security, runtime, observability, database, and CI requirements that apply to **every phase** of `mango-mdu-service`.

This document is intentionally separate from the master requirements document at `docs/requirement.md`.

- The **master requirements document** at `docs/requirement.md` defines service scope, downstream boundaries, ownership rules, development phases, and whole-service roadmap.
- This **common requirements document** defines the cross-phase engineering and delivery rules that every MDU phase must follow.

Every detailed phase requirements document for MDU Service shall inherit these common requirements unless an approved architecture revision explicitly overrides a rule.

All unqualified `must` and `shall` statements in this document are normative. `May` is optional. This document does not rely on external attachments; repo-tracked documents are the reviewable source of truth.

---

# 1. Cross-Phase Delivery Requirements

1. Exhaustive automated CI/CD test cases are mandatory for every phase.
2. A phase is not complete unless all required CI jobs pass.
3. Every PR must rely on automated tests executed in CI as the primary test evidence.
4. Manual or local test output may be included only as supporting evidence and must not replace CI.
5. Code, OpenAPI, design documentation, and phase requirements must remain aligned for every implemented feature.
6. Placeholder scaffold behavior is not acceptable for any feature claimed as implemented in a phase.
7. A feature is not complete unless its API contract, auth behavior, error model, and observability behavior are documented.
8. Future work may be deferred, but deferred work must be explicitly marked as out of scope rather than silently omitted.
9. Every phase must preserve the ownership and source-of-truth boundaries defined in the master requirements document.
10. No phase may introduce hidden local ownership of domains already owned by downstream services unless an approved architecture revision explicitly changes the service boundary.

---

# 2. Required Common Modules

MDU Service must use approved Router Architects / OpenWiFi common modules, or approved equivalent internal wrappers around them, instead of duplicating shared infrastructure logic.

The service shall use approved common solutions for:

- `github.com/routerarchitects/ra-common-mods/logger`
- `github.com/routerarchitects/ra-common-mods/buildinfo`
- `github.com/routerarchitects/ow-common-mods/servicediscovery`
- `github.com/routerarchitects/ow-common-mods/servicerpc`

Kafka-related common modules are not universally mandatory for MDU in every phase. If a phase introduces Kafka-driven behavior, that phase must explicitly require and document the Kafka modules it depends on.

## 2.1 Implementation Rules

- Use `ra-common-mods/logger` for structured, context-aware logging.
- Use `ra-common-mods/buildinfo` for build version and commit metadata.
- Use `ow-common-mods/servicediscovery` for approved internal service discovery where applicable.
- Use `ow-common-mods/servicerpc` or an approved common HTTP wrapper for service-to-service calls.
- Use approved shared helpers for auth propagation, request tracing, and internal-caller metadata handling where available.
- Do not duplicate common infrastructure logic in handlers or individual downstream clients without a documented technical reason.

## 2.2 Required Common Capability Areas

The implementation must have an approved common solution for:

- structured logging
- build metadata
- service discovery
- service-to-service HTTP/RPC calls
- request correlation and tracing propagation
- common auth/header propagation behavior
- error-normalization helpers where applicable

---

# 3. API and Security Boundary

MDU Service is not identical to Billing Service in exposure model because MDU contains both UI-facing APIs and potentially internal-only APIs. Therefore, MDU must define and enforce API classes clearly.

## 3.1 API Classes

### 3.1.1 UI-Facing Authenticated APIs
These are the normal MDU APIs called by the frontend or user-context clients.

Rules:
- Must require validated user/session credentials.
- Must enforce scope-aware authorization.
- Must use standardized MDU error responses.
- Must not rely only on frontend-side hiding for authorization.
- Must not expose raw internal-only downstream contracts unless intentionally documented.

### 3.1.2 Internal Service APIs
These are private service-to-service orchestration endpoints, if introduced.

Rules:
- Must require approved internal authentication.
- Must reject unauthenticated public access.
- Must not be accidentally exposed as public UI endpoints.
- Must require clear caller identification and audit context where applicable.

### 3.1.3 Admin / Debug / Support APIs
These are operationally sensitive APIs.

Rules:
- Must be private/internal or tightly restricted.
- Must require an explicit authorization model.
- Must not leak backend internals to normal consumers.
- Must be excluded from standard UI-facing contracts unless intentionally exposed.

## 3.2 Authentication Model

### 3.2.1 User-Facing Authentication
User-facing MDU APIs must validate bearer tokens through OWSEC.

### 3.2.2 Internal Service Authentication
Internal-only APIs, if introduced, must use an approved internal authentication model such as:
- internal API key
- service identity headers
- approved internal caller metadata
- required actor/audit metadata where applicable

### 3.2.3 No Authentication Ambiguity
Every endpoint must have one clearly documented authentication mode or an explicitly documented multi-mode rule. No endpoint may have an unclear or implicit security posture.

## 3.3 Authorization Rules

MDU Service must enforce:
- resource-level authorization
- scope-level authorization
- action-level authorization
- role/profile restrictions where relevant

Authorization must be enforced in the service even if the UI hides unauthorized actions.

## 3.4 Frontend Token Boundary

If an endpoint is internal-only, a frontend bearer token must not be treated as sufficient authentication unless the endpoint is explicitly documented to support that mode.

## 3.5 Actor and Audit Headers

For internal or privileged mutation routes, actor and audit headers may be required for traceability and supportability.

These headers are not authentication credentials by themselves, but they may be mandatory metadata.

## 3.6 Rejection Rules

MDU Service must reject requests when:
- authentication credentials are missing
- credentials are invalid
- required internal caller metadata is missing
- required actor/audit metadata is missing
- the caller is outside the allowed authorization scope
- a frontend token is presented to an internal-only route without the required internal authentication model

---

# 4. Required Architecture and Layering Rules

MDU Service must preserve a clean service structure throughout all phases.

## 4.1 Required Layering

Implementation shall remain split into the following concerns:

- **Transport layer**
  - HTTP routing
  - request parsing
  - response encoding
  - transport-level status mapping

- **Middleware layer**
  - authentication
  - authorization context extraction
  - correlation ID handling
  - logging/tracing hooks
  - common request guards

- **Application / orchestration layer**
  - multi-service workflow coordination
  - business composition logic
  - scope-aware validations
  - response shaping

- **Integration layer**
  - OWSEC client
  - PROV client
  - Billing Service client
  - OWGW client
  - NW Topology Service client
  - OWANALYTICS client
  - any future approved downstream client

- **Persistence layer**
  - migrations
  - repositories for approved local state only

- **Support layer**
  - observability
  - health/readiness
  - background workers where approved
  - build metadata
  - config handling

## 4.2 Handler Thinness Rule

HTTP handlers must remain thin.

Handlers shall not contain:
- downstream orchestration logic
- multi-step workflow coordination
- business composition logic
- direct persistence branching beyond simple transport concerns

Handlers may:
- parse transport inputs
- call orchestration services
- return normalized responses/errors

## 4.3 Downstream Adapter Isolation Rule

Every downstream integration must be isolated behind explicit adapters or clients.

No phase shall scatter raw downstream request logic across handlers or unrelated packages.

## 4.4 No Hidden Ownership Rule

MDU Service shall not gradually accumulate hidden ownership through convenience persistence.

If a phase needs local state, that state must be:
- explicitly justified
- documented in the phase requirements
- documented in the design doc
- represented in migrations
- consistent with the master service ownership model

---

# 5. Downstream Integration Requirements

Every phase of MDU Service must preserve correct integration behavior with approved downstream services.

## 5.1 Approved Downstream Systems

The downstream ecosystem currently includes:

- OWSEC
- PROV
- Billing Service
- OWGW
- NW Topology Service
- OWANALYTICS

## 5.2 Source-of-Truth Preservation Rule

MDU must not override source-of-truth ownership.

That means:
- OWSEC remains the source of truth for identity/session/admin-user lifecycle.
- PROV remains the source of truth for hierarchy, roles, policies, inventory ownership, configuration ownership, and RBAC persistence.
- Billing Service remains the source of truth for billing.
- OWGW remains the source of truth for live runtime device commands and runtime state.
- NW Topology Service remains the source of truth for topology computation.
- OWANALYTICS remains the source of truth for telemetry/time-series analytics.

## 5.3 Adapter Contract Rule

Every downstream integration must define:
- base URL or discovery pattern
- auth model
- timeout budget
- retry policy
- response normalization rules
- error translation rules
- observability/tracing behavior

## 5.4 No Raw Downstream Leakage

MDU must not leak raw downstream contracts directly to the UI unless:
- the behavior is explicitly intended
- the contract is stable enough
- the API is documented as such

## 5.5 Downstream Contract Quirk Rule

When a downstream service has non-standard or compatibility behavior, that behavior must be:
- documented
- handled in integration code
- covered by tests
- hidden from UI-facing routes where possible

---

# 6. Runtime and Database Requirements

## 6.1 Runtime Baseline

- MDU Service must be packaged as a Dockerized Go microservice.
- MDU Service must use PostgreSQL only for MDU-owned local state that is consistent with the active phase.
- The service must support containerized deployment, environment-based configuration, secret-based credential injection, health endpoints, and graceful startup/shutdown.
- Structured logs must be emitted to stdout/stderr.
- Build/version metadata must be visible through approved runtime/support surfaces.

## 6.2 Environment Configuration

The service must load and validate configuration for:

- service ports/listeners
- database
- OWSEC
- PROV
- Billing Service
- OWGW
- NW Topology Service
- OWANALYTICS
- auth settings
- internal credentials
- timeouts
- retry settings
- observability
- feature flags where approved

## 6.3 Fail-Fast Startup Rule

The service must fail startup when critical configuration is invalid or missing.

Silent fallback to unsafe defaults is not acceptable for:
- downstream base URLs
- credentials
- database connection
- required auth settings
- required internal security settings

## 6.4 Health and Readiness

The service must provide:
- liveness endpoint
- readiness endpoint
- dependency-aware readiness behavior where appropriate

## 6.5 Graceful Shutdown

The service must:
- stop accepting new traffic cleanly
- allow in-flight requests a bounded completion window
- stop background workers gracefully
- close database and downstream resources cleanly

## 6.6 Database Guardrails

The local database may be used for:
- migrations
- workflow metadata where approved
- audit logs where approved
- idempotency records where approved
- job state where approved
- bounded cache or support metadata where approved

The local database must not be used for authoritative tables for:
- RBAC truth
- hierarchy truth
- billing truth
- inventory truth
- configuration truth

Future-phase tables must not be introduced early unless the current phase actually requires them.

---

# 7. Docker Baseline

MDU Service Docker packaging shall follow a secure, minimal, production-grade Docker baseline. The implementation may adjust Go version, module path, command path, binary name, exposed port, or build arguments to match the final repository layout while preserving the required security and packaging properties.

```dockerfile
FROM golang:1.25-bookworm AS builder

WORKDIR /src

ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=dev

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -mod=mod \
  -ldflags "-s -w \
    -X github.com/routerarchitects/ra-common-mods/buildinfo.version=${BUILD_VERSION} \
    -X github.com/routerarchitects/ra-common-mods/buildinfo.commitHash=${BUILD_COMMIT}" \
  -o /out/mdu-service ./cmd/server

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/mdu-service /app/mdu-service

EXPOSE 17011

USER nonroot:nonroot
ENTRYPOINT ["/app/mdu-service"]
```

## 7.1 Required Docker Properties

The Docker build must ensure:

- multi-stage build
- no unnecessary toolchain in the final image
- binary built with production flags
- non-root execution
- correct port exposure for the final service
- predictable entrypoint
- compatibility with CI smoke-test runs

Docker image build and container startup smoke test must run in CI.

---

# 8. API Contract and Documentation Requirements

## 8.1 OpenAPI is Mandatory

Every production MDU endpoint must be represented in the service OpenAPI definition.

## 8.2 Contract Alignment Rule

Code, OpenAPI, and documentation must stay aligned.

A phase is incomplete if:
- routes exist without spec coverage
- spec exists without implemented behavior
- behavior differs from spec without an approved update

## 8.3 Request/Response Examples

Important APIs must include:
- example requests
- example success responses
- example error responses
- auth expectations
- scope/permission behavior notes where useful

## 8.4 Error Contract Documentation

The common error envelope and major error classes must be documented and reused consistently.

## 8.5 Downstream Mapping Notes

Critical downstream quirks and compatibility rules must be documented so they do not live only in code comments.

---

# 9. Observability Requirements

## 9.1 Logging

The service must provide:
- structured logs
- request-scoped logging
- correlation ID logging
- safe redaction of sensitive fields
- clear dependency failure visibility

## 9.2 Metrics

The service must provide metrics for:
- request count
- latency
- HTTP status distribution
- dependency call count
- dependency latency
- dependency failures
- retry count
- worker/job health where applicable
- degraded composition outcomes where applicable

## 9.3 Tracing

The service must provide distributed tracing across:
- incoming request handling
- downstream calls
- background jobs where applicable
- multi-step composed workflows

## 9.4 Auditability

Mutation workflows must be traceable through:
- caller identity
- endpoint/method
- target resource identifiers
- dependency touchpoints
- final status
- correlation identifiers

---

# 10. Reliability and Error Handling Requirements

## 10.1 Timeout Rules

Every downstream call must use explicit timeout budgets.

## 10.2 Retry Rules

Retries may be used only where safe.

Retry behavior must be:
- documented
- bounded
- observable
- idempotency-aware

## 10.3 Partial Composition Rule

For composed responses, partial failure behavior must be explicit.

If partial data can be returned, the contract must clearly indicate:
- what succeeded
- what failed
- whether the response is partial

## 10.4 Error Normalization

MDU must normalize downstream errors into stable MDU-facing error contracts.

Raw downstream errors may be logged safely, but must not be exposed verbatim as public API contracts unless explicitly intended.

## 10.5 No Silent Failure Rule

The service must not silently swallow dependency failures that affect user-visible correctness.

---

# 11. Required CI Checks

Required CI checks shall include:

- `go fmt` or formatting check
- `go vet`
- lint or static analysis if configured
- unit tests
- handler or API tests
- service-layer tests
- downstream adapter tests
- authentication and authorization middleware tests
- OpenAPI or schema validation where applicable
- PostgreSQL migration tests
- Docker build
- container startup smoke test

## 11.1 Expanded CI Checks by Phase

Later phases shall also add, where relevant:

- repository or database tests
- transaction and rollback tests
- concurrency and mutation-conflict tests
- batch-operation tests
- idempotency tests
- workflow/job tests
- reconciliation tests
- topology/analytics composition tests
- rollout/workflow durability tests
- integration tests against fake or test-double downstream services

---

# 12. Testing Requirements

## 12.1 Unit Testing

All core service logic must have unit tests.

## 12.2 Handler Testing

Transport behavior must be tested for:
- happy path
- validation failures
- auth failures
- downstream error mapping
- malformed request handling

## 12.3 Integration Testing

Adapters to downstream systems must have integration-style tests or high-confidence test doubles validating:
- request construction
- auth propagation
- timeout/retry behavior
- response parsing
- contract-quirk handling

## 12.4 Phase-Specific Workflow Testing

Each phase must add tests for the workflows it introduces.

## 12.5 Test Evidence Rule

CI remains the primary evidence of correctness. Local/manual logs are supporting evidence only.

---

# 13. Security Requirements

## 13.1 Secret Handling

Secrets must be loaded from environment configuration or approved secret-management systems.

Secrets must not be:
- hardcoded
- committed to the repo
- logged
- returned in APIs

## 13.2 Sensitive Logging Restrictions

The service must never log:
- raw passwords
- raw bearer tokens
- raw refresh tokens
- raw API keys
- sensitive downstream credentials
- sensitive personal data unless explicitly required and protected

## 13.3 TLS and Internal Service Security

The deployment model must support secure internal communication and TLS where required by the environment.

## 13.4 Least Privilege

MDU’s credentials to downstream systems must be least-privilege and environment-scoped.

## 13.5 Security Review Trigger

A design/security review must be required when a phase introduces:
- internal-only APIs
- privileged admin/debug endpoints
- async jobs touching multiple systems
- idempotency stores
- long-lived workflow persistence
- AI or automation execution hooks

---

# 14. Documentation Ownership Rules

## 14.1 Required Documents

The service must maintain:

- master requirements document
- common requirements document
- phase-specific requirements documents
- design document
- OpenAPI document

## 14.2 Change Synchronization Rule

When implementation changes:
- the phase doc must be updated if phase scope changes
- the design doc must be updated if technical approach changes
- OpenAPI must be updated if contract changes
- common requirements must be updated if cross-phase rules change

## 14.3 No Orphaned Docs Rule

Docs that no longer represent reality must be updated or removed. Stale scaffold or placeholder docs must not remain misleadingly authoritative.

---

# 15. Release Readiness Rules

A phase or feature is not release-ready unless:

1. CI passes
2. the required docs are updated
3. OpenAPI matches implementation
4. auth and authorization behavior is tested
5. logging/metrics/tracing are present
6. error handling is normalized
7. Docker build and startup succeed
8. downstream integration behavior is validated for the feature
9. no placeholder implementation remains for the claimed scope

---

# 16. Final Common Rules

1. These common requirements apply to every MDU Service phase.
2. The master requirements document defines service scope and phase roadmap.
3. Detailed phase documents define phase-specific behavior.
4. This common requirements document defines the cross-phase engineering guardrails.
5. No phase may violate downstream source-of-truth ownership unless an approved architecture revision explicitly changes the service boundary.
6. MDU must remain an orchestration and aggregation service unless a later approved design assigns direct ownership of a new domain.
7. Production readiness requires contract clarity, CI evidence, security discipline, observability, and consistent implementation structure.
