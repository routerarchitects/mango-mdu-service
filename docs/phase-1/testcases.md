# Mango MDU Service — Phase 1 Test Cases: Policy Overview

## 1. Scope

This document specifies test cases for Phase 1 of `mango-mdu-service`:
1. Operational & System Endpoints (`/livez`, `/api/v1/system`)
2. Policy Overview Endpoint (`GET /api/v1/policies/{policyId}/overview`)

---

## 2. Test Cases Matrix

| Test ID | Description | Input / Precondition | Expected Status | Expected Behavior |
|:---|:---|:---|:---|:---|
| **TC-SYS-001** | Public Liveness probe | `GET /livez` (no auth) | `200 OK` | Returns empty body or OK status |
| **TC-SYS-002** | System diagnostics info | `GET /api/v1/system?command=info` with valid token | `200 OK` | Returns subsystem info JSON |
| **TC-SYS-003** | System diagnostics unauthorized | `GET /api/v1/system?command=info` without token | `401 Unauthorized` | Standard `ApiError` envelope |
| **TC-POL-001** | Get Policy Overview — Policy with active assignments | `GET /api/v1/policies/{id}/overview` with valid token; policy has 2 users and 3 roles across 1 entity and 2 venues | `200 OK` | `summary.usedByUsers == 2`, `summary.scopedAssignmentsCount == 3`, `summary.propertiesCount == 1`, `summary.venuesCount == 2`, `assignedUsers` has 2 entries |
| **TC-POL-002** | Get Policy Overview — Unassigned policy | `GET /api/v1/policies/{id}/overview` with valid token; policy has no roles | `200 OK` | All summary counters are `0`, `assignedUsers` is empty `[]` |
| **TC-POL-003** | Get Policy Overview — Target policy not found | `GET /api/v1/policies/non-existent-id/overview` with valid token | `404 Not Found` | `ApiError` with `errorCode: 404` |
| **TC-POL-004** | Get Policy Overview — Missing bearer token | `GET /api/v1/policies/{id}/overview` without `Authorization` header | `401 Unauthorized` | Rejection before downstream calls |
| **TC-POL-005** | Get Policy Overview — Invalid bearer token | `GET /api/v1/policies/{id}/overview` with invalid/expired token | `401 Unauthorized` | Rejection from OWSEC token validation |
| **TC-POL-006** | Get Policy Overview — Downstream PROV unreachable | `GET /api/v1/policies/{id}/overview` with valid token; PROV mock down | `503 Service Unavailable` | Graceful failure in `ApiError` envelope |
| **TC-POL-007** | Tracing header propagation | `GET /api/v1/policies/{id}/overview` with `X-Request-Id: req-1` and `X-Correlation-Id: corr-1` | `200 OK` | Outbound downstream requests to PROV & SEC include tracing headers; response echoes or includes them |
| **TC-POL-008** | CORS Preflight OPTIONS | `OPTIONS /api/v1/policies/{id}/overview` (browser preflight) | `204 No Content` or `200 OK` | Bypasses bearer auth middleware, returns CORS allow headers |
