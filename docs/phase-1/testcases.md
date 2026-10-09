# Mango MDU Service — Phase 1 Test Cases: Policy Overview

## 1. Scope

This document specifies test cases for Phase 1 of `mango-mdu-service`:
1. Operational & System Endpoints (`/livez`, `/api/v1/system`)
2. Policy Overview Endpoint (`GET /api/v1/policy/{id}/overview`)

---

## 2. Test Cases Matrix

| Test ID | Description | Input / Precondition | Expected Status | Expected Behavior |
|:---|:---|:---|:---|:---|
| **TC-SYS-001** | Public Liveness probe | `GET /livez` (no auth) | `200 OK` | Returns empty body or OK status |
| **TC-SYS-002** | System diagnostics info | `GET /api/v1/system?command=info` with valid token | `200 OK` | Returns `SystemInfoResponse` JSON conforming to OpenAPI schema (`version`, `uptime`, `start`, `os`, `processors`, `hostname`, `UI`, `certificates`) |
| **TC-SYS-003** | System diagnostics unauthorized | `GET /api/v1/system?command=info` without token or API key | `401 Unauthorized` | HTTP 401 status code with empty body (bare status response from common public auth middleware) |
| **TC-SYS-004** | System diagnostics resources | `GET /api/v1/system?command=resources` with valid token | `200 OK` | Returns `SystemResourcesResponse` JSON conforming to OpenAPI schema (`numberOfFileDescriptors`, `currRealMem`, `peakRealMem`, `currVirtMem`, `peakVirtMem`) |
| **TC-SYS-005** | System set log levels | `POST /api/v1/system` with valid token and body `{"command":"setloglevel","subsystems":[{"tag":"HTTP","value":"INFO"}]}` | `200 OK` | Returns `SystemCommandSuccessResponse` JSON conforming to OpenAPI schema (`Code: 0`, `Operation: "POST"`, `Details: "Command completed."`) |
| **TC-SYS-006** | System set log levels missing subsystems | `POST /api/v1/system` with valid token and body `{"command":"setloglevel"}` (missing or empty `subsystems`) | `400 Bad Request` | Returns `ApiError` envelope with `ErrorCode: 400` ("Invalid or missing parameters") |
| **TC-POL-001** | Get Policy Overview — Policy with active assignments across multiple scopes | `GET /api/v1/policy/{id}/overview` with valid token; policy has 2 unique users across 3 roles, 1 entity, and 2 venues (User 1 assigned to 2 venues, User 2 assigned to whole property) | `200 OK` | `totalUsers == 2`, `totalScopedAssignments == 3`, `totalProperties == 1`, `totalVenues == 2`, `policy.name` populated, `usersWithPolicy` has 2 unique user entries; User 1 has `scopes` length 2 ("Tower A" and "Tower B"), User 2 has `scopes` length 1 ("All venues"); total scope items across users equals 3 |
| **TC-POL-002** | Get Policy Overview — Unassigned policy | `GET /api/v1/policy/{id}/overview` with valid token; policy has no roles | `200 OK` | All summary counters are `0` (`totalUsers == 0`, `totalScopedAssignments == 0`, `totalProperties == 0`, `totalVenues == 0`), `usersWithPolicy` is empty `[]` |
| **TC-POL-003** | Get Policy Overview — Target policy not found (valid UUID) | `GET /api/v1/policy/00000000-0000-4000-8000-000000000001/overview` with valid token | `404 Not Found` | Valid UUID not present in OWPROV returns `ApiError` with `ErrorCode: 404` |
| **TC-POL-004** | Get Policy Overview — Missing bearer token | `GET /api/v1/policy/{id}/overview` without `Authorization` header | `401 Unauthorized` | Rejection before downstream calls |
| **TC-POL-005** | Get Policy Overview — Invalid bearer token | `GET /api/v1/policy/{id}/overview` with invalid/expired token | `401 Unauthorized` | Rejection from OWSEC token validation |
| **TC-POL-006** | Get Policy Overview — Downstream PROV unreachable | `GET /api/v1/policy/{id}/overview` with valid token; PROV mock down | `503 Service Unavailable` | Graceful failure in `ApiError` envelope with `ErrorCode: 503` |
| **TC-POL-007** | Downstream header, tracing, and filter propagation | `GET /api/v1/policy/{id}/overview` with `X-Request-Id: req-1` and `X-Correlation-Id: corr-1` | `200 OK` | Outbound requests propagate `Authorization: Bearer <owsec-token>`, `User-Agent: mango-mdu-service/1.0`, and tracing headers (`X-Request-Id`, `X-Correlation-Id`); OWPROV requests omit `X-API-KEY` (role query specifies `?policyId={id}`), while OWSEC private `/api/v1/users` requests include dual authentication (`X-INTERNAL-NAME` and `X-API-KEY`) |
| **TC-POL-008** | CORS Preflight OPTIONS | `OPTIONS /api/v1/policy/{id}/overview` with `Access-Control-Request-Headers: Authorization, X-Request-Id, X-Correlation-Id` | `204 No Content` or `200 OK` | Phase 1 implementation acceptance requirement (expected to fail until CORS middleware implementation is updated in the code implementation PR): Bypasses bearer auth middleware; `Access-Control-Allow-Headers` includes `Authorization`, `X-Request-Id`, and `X-Correlation-Id` |
| **TC-POL-009** | Get Policy Overview — Malformed policy UUID | `GET /api/v1/policy/not-a-uuid/overview` with valid token | `400 Bad Request` | Immediate validation failure before downstream call; returns `ApiError` with `ErrorCode: 400` |
| **TC-POL-010** | Get Policy Overview — Unmatched / unresolvable user in OWPROV role | `GET /api/v1/policy/{id}/overview` with valid token; role references user UUID not returned in requester's OWSEC user set | `200 OK` | Graceful skip: unmatched/unresolvable user is simply skipped (no synthetic `"Deleted User"` entry generated); role remains counted in `totalScopedAssignments`; `totalUsers` reflects only unique matched users (`usersWithPolicy.length == totalUsers`) |

