# Service Design: mango-mdu-service

This document details the architectural layout, database models, and internal logic flows for the **mango-mdu-service** microservice.

---

## 1. High-Level Architecture

The current checked-in baseline uses a dual-port TLS HTTP runtime, shared auth middleware, PostgreSQL-backed startup/migration wiring, and optional service discovery / RPC integrations.

```text
[External Caller] ---> (Public TLS Port 16010) ---> [http.Module] ---> [public auth middleware] ---> [/livez, /api/v1/system]

[Internal Caller] ---> (Private TLS Port 17010) --> [http.Module] ---> [private auth middleware] --> [/livez, /api/v1/system]

[Bootstrap / Runtime] --> [db.Database migrations]
                       --> [service discovery]
                       --> [service RPC / OWSEC client]
```

---

## 2. Logical Data Model

No Mango-domain entity model is committed in the current baseline. MDU-facing domain models for users, operators, entities, venues, roles, policies, and related orchestration views should be introduced only with the corresponding Phase 1 implementation work.

---

## 3. Database Schema

The current checked-in migration baseline does not define any approved MDU-owned business tables. New schema should be added only when a Phase 1 workflow requires local state that does not duplicate downstream source-of-truth ownership.

---

## 4. REST API Endpoints Contract

The current runtime baseline exposes:

- `GET /livez` on both public and private ports without authentication
- `/api/v1/system` diagnostics routes on both public and private ports through the shared subsystem/system-routes module

Refer to `docs/openapi.yaml` for the checked-in contract baseline. Mango-facing `/api/v1/mdu/*` business APIs should be documented as they are implemented.

---

## 5. Background Routines & Concurrency

The service may run:

- database startup migration checks
- service discovery publisher lifecycle
- service RPC / OWSEC client initialization
- HTTP listener goroutines for public and private TLS servers

---

## 6. Error Codes Matrix

Document all custom errors returned by the service under the `apperror` codes.

| Code Name | HTTP Status | Description |
|---|---|---|
| `CodeUnauthorized` | 401 | Authentication credentials are missing or invalid. |
| `CodeForbidden` | 403 | The caller is authenticated but not allowed to perform the operation. |
| `CodeInvalidInput` | 400 | Request payload or command validation failed. |
| `CodeInternal` | 500 | Runtime startup, dependency, or internal execution failure occurred. |
