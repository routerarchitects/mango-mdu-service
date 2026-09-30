# Mango MDU Service — Phase 1 Workflow: Policy Overview

## 1. Overview

This document describes the runtime execution flow for Phase 1 of `mango-mdu-service`.

The focus of this phase is delivering the **Policy Overview API** to power the `Users & Access -> Policies -> Overview` tab in `mango-operator-ui`, alongside core operational system routes (`/livez`, `/api/v1/system`).

---

## 2. Policy Overview Workflow (`GET /api/v1/policies/{policyId}/overview`)

### Step 1: Inbound Request Handling
- The Operator UI sends `GET /api/v1/policies/{policyId}/overview` with an `Authorization: Bearer <token>` header.
- Optional tracing headers (`X-Request-Id`, `X-Correlation-Id`) are parsed or generated if absent.
- The authentication middleware validates the bearer token against OWSEC (`AUTH_ENABLED=true`).
- If token validation fails, a normalized `401 Unauthorized` is returned immediately.

### Step 2: Policy Details Resolution
- MDU calls OWPROV: `GET /api/v1/managementPolicy/{policyId}` forwarding the user context.
- If the policy does not exist in OWPROV, MDU returns `404 Not Found`.
- Policy metadata (Name, Description, Modified/Created timestamp) is extracted.

### Step 3: Management Roles & Scope Aggregation
- MDU calls OWPROV: `GET /api/v1/managementRole?limit=1000`.
- MDU filters all management roles where the role references `policyId` (by matching role's `managementPolicy` ID or name).
- For each matching role:
  - Collects all user IDs in `role.users[]`.
  - Collects the entity ID (`role.entity`) and venue ID (`role.venue`).
- Calculates the aggregate summary counts:
  - `usedByUsers`: count of unique user IDs.
  - `scopedAssignmentsCount`: count of matching management roles.
  - `propertiesCount`: count of unique non-empty entity IDs.
  - `venuesCount`: count of unique non-empty venue IDs.

### Step 4: User & Scope Entity Enrichment
- For each assigned user:
  - Resolves user display name and email (from OWSEC user directory or role data).
  - Resolves property name from OWPROV entity directory.
  - Resolves venue scope name ("Whole property" if venue is empty, or the specific venue name).
- Populates the `assignedUsers[]` list.

### Step 5: Response Composition
- Formats the consolidated response according to `docs/phase-1/mango-mdu-openapi.yaml`.
- Returns `200 OK` with JSON payload.
