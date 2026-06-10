# MDU API Flow Document

## 1. Purpose

This document defines the API flow for the MDU service and its integration with OWPROV.

It combines the following areas:

```text
1. MDU -> OWPROV API flow
2. Customer ownership / RBAC flow
3. Saga execution flow for create, update, delete, and venue import
4. Compensation and retry mechanism
5. Recursive venue/entity pagination flow
```

The main rule is:

```text
MDU is the public API and Saga orchestrator.
OWPROV is the provisioning/source service for entities and venues.
MDU stores customer ownership and local customer data.
```

---

# 2. Service Responsibilities

## 2.1 MDU Service

MDU is responsible for:

```text
Customer APIs
Customer ownership / RBAC
Calling OWPROV APIs
Saga orchestration
Compensation retries
Idempotency handling
Pagination wrapper for recursive OWPROV data
```

MDU owns:

```text
mdu_customers
saga_executions
saga_steps
saga_step_compensations
optional venue import tracking/cache tables
```

## 2.2 OWPROV Service

OWPROV is responsible for:

```text
Entity creation
Entity deletion
Venue creation
Venue deletion
Venue/entity fetch APIs
Configuration/resource binding if required
```

MDU should call OWPROV using a service client.

---

# 3. Authentication and RBAC

## 3.1 Authentication

Every MDU public endpoint must require:

```http
Authorization: Bearer <owsec-token>
```

The token must be validated by MDU middleware.

After validation, middleware must extract user identity:

```json
{
  "userId": "usr_01J...",
  "email": "operator@example.com",
  "roles": ["mdu_operator"]
}
```

The middleware should place this in request context:

```go
type AuthContext struct {
    UserID string
    Email  string
    Roles  []string
}
```

## 3.2 Customer Ownership Rule

Every customer must belong to the user who created it.

```text
customer.created_by_user_id = loggedInUser.userId
```

All customer-scoped APIs must check ownership before processing.

If the customer does not belong to the logged-in user, return:

```json
{
  "error": {
    "code": "CUSTOMER_NOT_FOUND",
    "message": "Customer not found."
  }
}
```

Do not return `403 customer belongs to another user`, because that leaks existence of the resource.

---

# 4. Customer Table Schema

```sql
CREATE TABLE mdu_customers (
  id VARCHAR(40) PRIMARY KEY,

  entity_id VARCHAR(100) NOT NULL,

  created_by_user_id VARCHAR(100) NOT NULL,
  created_by_email VARCHAR(255) NULL,

  name VARCHAR(255) NOT NULL,
  phone_number VARCHAR(30) NULL,

  location_json JSONB NOT NULL,

  status VARCHAR(40) NOT NULL DEFAULT 'active',

  deleted_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

  UNIQUE (entity_id)
);
```

## 4.1 Customer Status Values

```text
active
deleting
delete_failed
deleted
```

---

# 5. Saga Database Schema

## 5.1 `saga_executions`

One row represents one full Saga operation.

```sql
CREATE TABLE saga_executions (
  id VARCHAR(40) PRIMARY KEY,

  saga_type VARCHAR(100) NOT NULL,
  indempotency_key VARCHAR(255) NOT NULL UNIQUE,

  status VARCHAR(40) NOT NULL,
  current_step VARCHAR(100) NULL,
  failed_step VARCHAR(100) NULL,

  request_json JSONB NULL,
  response_json JSONB NULL,
  error_json JSONB NULL,
  error_message TEXT NULL,

  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

Recommended status values:

```text
pending/running
completed
failed
manual_intervention_required
```

## 5.2 `saga_steps`

One row represents one forward execution step.

```sql
CREATE TABLE saga_steps (
  id VARCHAR(40) PRIMARY KEY,

  saga_id VARCHAR(40) NOT NULL,

  step_no INT NOT NULL,
  step_name VARCHAR(100) NOT NULL,
  status VARCHAR(40) NOT NULL,

  resource_type VARCHAR(100) NULL,
  resource_id VARCHAR(100) NULL,

  request_json JSONB NULL,
  response_json JSONB NULL,
  error_json JSONB NULL,

  before_json JSONB NULL,
  after_json JSONB NULL,

  compensation_required BOOLEAN NOT NULL DEFAULT FALSE,

  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

  UNIQUE (saga_id, step_no),

  CONSTRAINT fk_saga_steps_execution
    FOREIGN KEY (saga_id)
    REFERENCES saga_executions(id)
    ON DELETE CASCADE
);
```

Recommended status values:

```text
pending
completed
failed
manual_interventaion_required
```

## 5.3 `saga_step_compensations`

One row represents retryable compensation or forward-recovery work.

```sql
CREATE TABLE saga_step_compensations (
  id VARCHAR(40) PRIMARY KEY,

  saga_id VARCHAR(40) NOT NULL,
  saga_step_id VARCHAR(40) NOT NULL,

  compensation_type VARCHAR(100) NOT NULL,
  compensation_status VARCHAR(50) NOT NULL DEFAULT 'pending',

  resource_type VARCHAR(100) NOT NULL,
  resource_id VARCHAR(100) NOT NULL,

  request_json JSONB NULL,
  response_json JSONB NULL,
  error_json JSONB NULL,
  error_message TEXT NULL,

  retry_count INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 10,

  next_retry_at TIMESTAMP NULL,
  last_attempt_at TIMESTAMP NULL,
  completed_at TIMESTAMP NULL,

  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

  CONSTRAINT fk_saga_step_compensations_saga
    FOREIGN KEY (saga_id)
    REFERENCES saga_executions(id)
    ON DELETE CASCADE,

  CONSTRAINT fk_saga_step_compensations_step
    FOREIGN KEY (saga_step_id)
    REFERENCES saga_steps(id)
    ON DELETE CASCADE
);
```

Recommended compensation status values:

```text
pending
running
successful
failed
manual_intervention_required
```

## 5.4 Saga Indexes

```sql
CREATE INDEX idx_saga_executions_status
ON saga_executions(status);

CREATE INDEX idx_saga_executions_type_status
ON saga_executions(saga_type, status);

CREATE INDEX idx_saga_steps_saga_id
ON saga_steps(saga_id);

CREATE INDEX idx_saga_step_compensations_due
ON saga_step_compensations(compensation_status, next_retry_at);

CREATE INDEX idx_saga_step_compensations_saga_id
ON saga_step_compensations(saga_id);

CREATE INDEX idx_saga_step_compensations_step_id
ON saga_step_compensations(saga_step_id);
```

Optional uniqueness:

```sql
CREATE UNIQUE INDEX uq_saga_step_compensation_type
ON saga_step_compensations(saga_step_id, compensation_type);
```

---

# 6. Idempotency Key

Every create/update/delete/import API should accept or create a request key.

Use:

```http
Idempotency-Key: <unique-key>
```

Purpose:

```text
If UI retries the same request, MDU should not duplicate work.
```

Rules:

```text
Same user action retry -> same key
New user action -> new key
```

CREATE TABLE mdu_idempotency_keys (
  key VARCHAR(255) PRIMARY KEY,

  request_hash VARCHAR(128) NOT NULL,

  status VARCHAR(40) NOT NULL,

  response_json JSONB NULL,
  error_json JSONB NULL,

  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

);
```

MDU should store this in `mdu_idempotency_keys.indempotency_key`.

If the same `indempotency_key` is received again:

```text
completed -> return previous response_json
running -> return operation status
failed/compensated -> return previous error/compensation result
```

---

# 7. Endpoint Summary

| Method | Endpoint | Purpose | Source of Truth |
|---|---|---|---|
| `POST` | `/api/v1/mdu/customers` | Create customer after creating OWPROV entity | MDU + OWPROV |
| `GET` | `/api/v1/mdu/customers` | List owned customers | MDU DB |
| `GET` | `/api/v1/mdu/customers/{customerId}` | Get owned customer | MDU DB |
| `PATCH` | `/api/v1/mdu/customers/{customerId}` | Update customer/entity | MDU + OWPROV |
| `DELETE` | `/api/v1/mdu/customers/{customerId}?mode=soft` | Soft delete customer | MDU + OWPROV |
| `DELETE` | `/api/v1/mdu/customers/{customerId}?mode=hard` | Hard delete customer | MDU + OWPROV |
| `GET` | `/api/v1/mdu/customers/{customerId}/entity` | Get associated OWPROV entity | OWPROV |
| `POST` | `/api/v1/mdu/customers/{customerId}/venues/import` | Import nested venue JSON | OWPROV + MDU Saga |
| `GET` | `/api/v1/mdu/customers/{customerId}/venues` | Get venues with pagination/tree mode | OWPROV/MDU wrapper |
| `GET` | `/api/v1/mdu/customers/{customerId}/venues/{venueId}` | Get venue or subtree | OWPROV/MDU wrapper |
| `PATCH` | `/api/v1/mdu/customers/{customerId}/venues/{venueId}` | Update venue | OWPROV |
| `DELETE` | `/api/v1/mdu/customers/{customerId}/venues/{venueId}` | Delete venue | OWPROV |

---

# 8. Route Registration Example

```go
// Customer APIs
api.Post("/customers", customerHandler.CreateCustomer)
api.Get("/customers", customerHandler.ListCustomers)
api.Get("/customers/:customerId", customerHandler.GetCustomer)
api.Patch("/customers/:customerId", customerHandler.UpdateCustomer)
api.Delete("/customers/:customerId", customerHandler.DeleteCustomer)

// Customer Entity API
api.Get("/customers/:customerId/entity", entityHandler.GetEntity)

// Venue APIs
api.Post("/customers/:customerId/venues/import", venueHandler.ImportVenues)
api.Get("/customers/:customerId/venues", venueHandler.ListVenues)     //will fetch all the data
api.Get("/customers/:customerId/venues/:venueId", venueHandler.GetVenueById) //it can be used to paginate data
api.Patch("/customers/:customerId/venues/:venueId", venueHandler.UpdateVenueById)
api.Delete("/customers/:customerId/venues/:venueId", venueHandler.DeleteVenueById)

// Optional operation APIs
api.Get("/operations/:operationId", operationHandler.GetOperation)
api.Post("/operations/:operationId/retry", operationHandler.RetryOperation)
api.Post("/operations/:operationId/compensate", operationHandler.CompensateOperation)
```

---

# 9. Create Customer API Flow

## 9.1 Public API

```http
POST /api/v1/mdu/customers
Authorization: Bearer <owsec-token>
Idempotency-Key: mdu-customer-create:<uuid>
Content-Type: application/json
```

Request:

```json
{
  "name": "ra",
  "phoneNumber": "+919876543210",
  "location": {
    "friendlyName": "RA Main Location",
    "addressLine1": "Tower Road",
    "city": "Bengaluru",
    "state": "Karnataka",
    "country": "India",
    "postalCode": "560001",
    "latitude": 12.9716,
    "longitude": 77.5946
  }
}
```

## 9.2 OWPROV API Called by MDU

```http
POST https://openwifi.wlan.local:16005/api/v1/entity/0
Authorization: Bearer <owprov-token>
Idempotency-Key: mdu-customer-create:<uuid>:owprov-entity
Content-Type: application/json
```

Body:

```json
{
  "name": "ra",
  "deviceRules": {
    "rrm": "inherit",
    "rcOnly": "inherit",
    "firmwareUpgrade": "inherit"
  },
  "description": "",
  "parent": "0000-0000-0000",
  "sourceIP": []
}
```

OWPROV returns:

```json
{
  "id": "849242cf-466a-449c-bece-08b8b0980f80",
  "name": "ra",
  "parent": "0000-0000-0000"
}
```

MDU stores:

```text
mdu_customers.entity_id = response.id
mdu_customers.created_by_user_id = loggedInUser.userId
```

## 9.3 Saga Type

```text
create_customer
```

## 9.4 Saga Steps

| Step No. | Step Name | Action |
|---:|---|---|
| 1 | `validate_request` | Validate token, payload, and indempotency key |
| 2 | `create_owprov_entity` | Call OWPROV `/api/v1/entity/0` |
| 3 | `create_mdu_customer` | Insert local customer with `entityId` and `created_by_user_id` |
| 4 | `complete_saga` | Store response and mark completed |

## 9.5 Failure Handling

| Failure Point | What Was Created? | Action |
|---|---|---|
| Validation fails | Nothing | Saga failed |
| OWPROV entity creation fails | Nothing local | Saga failed |
| OWPROV succeeds, MDU insert fails | OWPROV entity exists | Create compensation to delete OWPROV entity |
| OWPROV entity delete compensation fails | Entity may remain | Retry via background compensation worker |
| Response fails after success | Entity and customer exist | Retry same key returns previous success |

Compensation row for failed local insert:

```text
compensation_type = delete_owprov_entity
resource_type = owprov_entity
resource_id = <entityId>
```

---

# 10. Customer Ownership Queries

## 10.1 List Customers

```sql
SELECT *
FROM mdu_customers
WHERE created_by_user_id = $1
  AND status <> 'deleted'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

## 10.2 Get Customer

```sql
SELECT *
FROM mdu_customers
WHERE id = $1
  AND created_by_user_id = $2
  AND status <> 'deleted';
```

## 10.3 Update/Delete/Venue APIs

Before any customer-scoped operation:

```sql
SELECT id, entity_id, status
FROM mdu_customers
WHERE id = $1
  AND created_by_user_id = $2
  AND status <> 'deleted';
```

---

# 11. Update Customer API Flow

## 11.1 API

```http
PATCH /api/v1/mdu/customers/{customerId}
Authorization: Bearer <owsec-token>
Idempotency-Key: customer-update:<uuid>
Content-Type: application/json
```

## 11.2 Saga Type

```text
update_customer
```

## 11.3 Saga Steps

| Step No. | Step Name | Action |
|---:|---|---|
| 1 | `validate_request` | Validate token, ownership, payload |
| 2 | `read_current_customer_state` | Read local customer and store `before_json` |
| 3 | `read_current_owprov_entity_state` | Read OWPROV entity and store `before_json` |
| 4 | `patch_owprov_entity` | Patch OWPROV entity if required |
| 5 | `patch_mdu_customer` | Update local customer |
| 6 | `complete_saga` | Mark completed |

## 11.4 PATCH Compensation

For PATCH operations, compensation means:

```text
Restore old state using before_json.
```

If OWPROV entity is patched but local customer update fails:

```text
compensation_type = restore_owprov_entity
resource_type = owprov_entity
resource_id = entityId
request_json = old OWPROV entity state
```

The worker retries:

```http
PATCH /api/v1/entity/{entityId}
Content-Type: application/json
```

with the old `before_json`.

---

# 12. Venue Import API Flow

## 12.1 API

```http
POST /api/v1/mdu/customers/{customerId}/venues/import
Authorization: Bearer <owsec-token>
Idempotency-Key: venue-import:<uuid>
Content-Type: application/json
```

Request:

```json
{
  "entityId": "849242cf-466a-449c-bece-08b8b0980f80",
  "venues": [
    {
      "friendlyName": "Sunrise Residency",
      "kind": "venue",
      "externalReference": "venue-root",
      "children": [
        {
          "friendlyName": "Tower A",
          "kind": "tower",
          "externalReference": "tower-a",
          "children": [
            {
              "friendlyName": "Floor 1",
              "kind": "floor",
              "externalReference": "tower-a-floor-1",
              "children": []
            }
          ]
        }
      ]
    }
  ]
}
```

## 12.2 MDU Pre-Checks

```text
1. Validate owsec token.
2. Extract userId.
3. Load customer using customerId + userId.
4. Validate request.entityId == customer.entity_id.
5. Only then call OWPROV.
```

## 12.3 OWPROV API Called by MDU

```http
POST /api/v1/entities/{entityId}/venues/import
Authorization: Bearer <owprov-token>
Idempotency-Key: venue-import:<uuid>:owprov
Content-Type: application/json

NOTE : currently it does not exist in owprov but it will be implemented soon
```

## 12.4 Success Response From OWPROV

```json
{
  "data": {
    "entityId": "849242cf-466a-449c-bece-08b8b0980f80",
    "status": "completed",
    "createdVenueIds": [
      "ven_root_01J",
      "ven_tower_a_01J",
      "ven_floor_1_01J"
    ],
    "venuesCreated": 3
  }
}
```

## 12.5 Partial Failure Response From OWPROV

```json
{
  "success": false,
  "error": {
    "code": "VENUE_IMPORT_PARTIAL_FAILED",
    "message": "Venue creation failed at /Sunrise Residency/Tower A/Floor 2"
  },
  "data": {
    "entityId": "849242cf-466a-449c-bece-08b8b0980f80",
    "status": "partial_failed",
    "createdVenueIds": [
      "ven_root_01J",
      "ven_tower_a_01J",
      "ven_floor_1_01J"
    ],
    "failedAt": "/Sunrise Residency/Tower A/Floor 2"
  }
}
```

## 12.6 Saga Type

```text
import_customer_venues
```

## 12.7 Saga Steps

| Step No. | Step Name | Action |
|---:|---|---|
| 1 | `validate_request` | Validate token, ownership, entityId, venue JSON |
| 2 | `load_customer` | Load customer and entityId |
| 3 | `validate_entity_ownership` | Check request entityId equals customer entityId |
| 4 | `create_venue_import_record` | Create local import tracking row if used |
| 5 | `call_owprov_import_venues` | Send nested JSON to OWPROV |
| 6 | `store_created_venue_ids` | Store returned venue IDs |
| 7 | `complete_import` | Mark import completed |

## 12.8 Compensation on Partial Failure

If OWPROV returns partial failure with `createdVenueIds`, MDU must delete those venue IDs in reverse order.

```text
DELETE ven_floor_1_01J
DELETE ven_tower_a_01J
DELETE ven_root_01J
```

OWPROV delete API:

```http
DELETE /api/v1/venues/{venueId}
```

If any delete fails, create/update compensation row:

```text
compensation_type = delete_owprov_venue
resource_type = owprov_venue
resource_id = venueId
```

Background worker keeps retrying until successful or manual intervention is required.

---

# 13. Update Venue API Flow

## 13.1 API

```http
PATCH /api/v1/mdu/customers/{customerId}/venues/{venueId}
Authorization: Bearer <owsec-token>
Idempotency-Key: venue-update:<uuid>
Content-Type: application/json
```

## 13.2 Saga Type

```text
update_customer_venue
```

## 13.3 Saga Steps

| Step No. | Step Name | Action |
|---:|---|---|
| 1 | `validate_request` | Validate token, ownership, venueId, payload |
| 2 | `load_customer_and_entity_id` | Load customer and entityId |
| 3 | `read_current_owprov_venue_state` | GET current venue and store `before_json` |
| 4 | `patch_owprov_venue` | PATCH venue in OWPROV |
| 5 | `update_local_venue_cache` | Optional; skip if not used |
| 6 | `complete_saga` | Mark Saga completed |

## 13.4 Compensation

If OWPROV venue is patched and a later critical step fails:

```text
Restore old OWPROV venue state using before_json.
```

Compensation:

```http
PATCH /api/v1/venues/{venueId}
Content-Type: application/json
```

with old venue JSON.

---

# 14. Customer Delete Flow

Customer delete supports:

```text
soft delete
hard delete
```

For both flows, the safest order is:

```text
1. Delete OWPROV entity first.
2. Then delete or mark MDU customer.
```

## 14.1 Soft Delete API

```http
DELETE /api/v1/mdu/customers/{customerId}?mode=soft
Authorization: Bearer <owsec-token>
Idempotency-Key: customer-soft-delete:<uuid>
```

### Soft Delete Saga Steps

| Step No. | Step Name | Action |
|---:|---|---|
| 1 | `validate_request` | Validate token, ownership, customerId |
| 2 | `load_customer` | Load customer and entityId |
| 3 | `mark_customer_deleting` | Set customer status = `deleting` |
| 4 | `delete_owprov_entity` | Delete OWPROV entity |
| 5 | `soft_delete_mdu_customer` | Set customer status = `deleted` |
| 6 | `complete_saga` | Mark Saga completed |

If OWPROV delete fails:

```text
customer.status = delete_failed
compensation_type = delete_owprov_entity
background worker retries
```

## 14.2 Hard Delete API

```http
DELETE /api/v1/mdu/customers/{customerId}?mode=hard
Authorization: Bearer <owsec-token>
Idempotency-Key: customer-hard-delete:<uuid>
```

### Hard Delete Saga Steps

| Step No. | Step Name | Action |
|---:|---|---|
| 1 | `validate_request` | Validate token, ownership, customerId |
| 2 | `load_customer_snapshot` | Store full customer snapshot in Saga |
| 3 | `mark_customer_deleting` | Set customer status = `deleting` |
| 4 | `delete_owprov_entity` | Delete OWPROV entity |
| 5 | `hard_delete_mdu_customer` | Physically delete local customer row |
| 6 | `complete_saga` | Mark Saga completed |

If OWPROV delete fails:

```text
Do not hard delete customer.
Set customer.status = delete_failed.
Schedule delete_owprov_entity compensation.
```

If local hard delete fails after OWPROV delete succeeds:

```text
Schedule hard_delete_mdu_customer retry.
Do not recreate OWPROV entity.
```

This is forward recovery, not rollback.

---

# 15. Compensation Worker

The compensation worker runs:

```text
1. On MDU server startup
2. Periodically, for example every 30 seconds
```

## 15.1 Worker Query

```sql
SELECT *
FROM saga_step_compensations
WHERE compensation_status IN ('pending', 'failed')
  AND (
    next_retry_at IS NULL
    OR next_retry_at <= NOW()
  )
ORDER BY created_at ASC
LIMIT 50
FOR UPDATE SKIP LOCKED;
```

## 15.2 Supported Compensation Types

| Compensation Type | Action |
|---|---|
| `delete_owprov_entity` | Call OWPROV `DELETE /api/v1/entities/{entityId}` |
| `restore_owprov_entity` | PATCH entity back to old `before_json` |
| `delete_owprov_venue` | Call OWPROV `DELETE /entities/{entityId}/venues/{venueId}` |
| `restore_owprov_venue` | PATCH venue back to old `before_json` |
| `hard_delete_mdu_customer` | Run local hard delete |
| `soft_delete_mdu_customer` | Set customer status = `deleted` |

## 15.3 Backoff Strategy

```text
Retry 1  -> 1 minute
Retry 2  -> 2 minutes
Retry 3  -> 5 minutes
Retry 4  -> 10 minutes
Retry 5  -> 30 minutes
Retry 6  -> 1 hour
Retry 7  -> 2 hours
Retry 8  -> 6 hours
Retry 9  -> 12 hours
Retry 10 -> 24 hours
```

If max retries are exhausted:

```text
compensation_status = manual_intervention_required
saga.status = manual_intervention_required
```

## 15.4 Idempotent Compensation Rule

Delete operations should treat these as success:

```text
200 OK
204 No Content
404 Not Found
```

Because the desired final state is:

```text
Resource does not exist.
```

---


## 16. Recursive Data and Pagination Flow

Recursive venue/entity data should not be returned as unlimited nested JSON.

For the MDU UI, use only two APIs:

```http
GET /api/v1/mdu/customers/{customerId}/venues
GET /api/v1/mdu/customers/{customerId}/venues/{parentId}/children?page=1&pageSize=50
```

The first API returns the customer-level venue/entity data.

The second API returns paginated children for a selected parent.

---

## 1. API 1: Get All Data by Customer ID

```http
GET /api/v1/mdu/customers/{customerId}/venues
Authorization: Bearer <owsec-token>
```

### Purpose

This API returns the main venue/entity data for a customer.

MDU should use `customerId` to:

```text
1. Validate logged-in user ownership.
2. Fetch customer from `mdu_customers`.
3. Get `entityId` from customer row.
4. Fetch entity and root venue data from OWPROV or MDU venue cache.
5. Return root-level data with child counts.
```

This API should not return unlimited deep recursive children.

It should return enough data to render the first page/root level of the tree.

### MDU Internal Flow

```text
1. Validate owsec token.
2. Extract userId from token.
3. Fetch customer:

   SELECT id, entity_id
   FROM mdu_customers
   WHERE id = :customerId
     AND created_by_user_id = :userId
     AND status <> 'deleted';

4. If customer not found:
   return CUSTOMER_NOT_FOUND.

5. Use customer.entity_id.

6. Fetch entity details from OWPROV:

   GET /api/v1/entity/{entityId}

7. Read entity.venues or root venue IDs.

8. Fetch root venue details from OWPROV or MDU local venue cache.

9. For every root venue, calculate:
   - hasChildren
   - childCount from children array

10. Return root-level data.
```

### Example Response

```json
{
  "data": {
    "customerId": "cus_01J...",
    "entityId": "849242cf-466a-449c-bece-08b8b0980f80",
    "customerName": "RA Customer",
    "rootVenues": [
      {
        "venueId": "ven_root_01J",
        "friendlyName": "Sunrise Residency",
        "kind": "venue",
        "hasChildren": true,
        "childCount": 2
      }
    ]
  },
  "meta": {
    "scope": "root",
    "totalRootVenues": 1
  }
}
```

---

## 2. API 2: Get Children by Parent ID

```http
GET /api/v1/mdu/customers/{customerId}/venues/{parentId}/children?page=1&pageSize=50
Authorization: Bearer <owsec-token>
```

### Purpose

This API returns paginated children of a specific parent venue.

Use this when the UI expands a node in the tree.

The `childCount` should be calculated from the `children` array stored in the venue data.

Example:

```json
{
  "venueId": "ven_root_01J",
  "children": [
    "ven_tower_a_01J",
    "ven_tower_b_01J"
  ]
}
```

Then:

```text
childCount = len(children)
```

### MDU Internal Flow

```text
1. Validate owsec token.
2. Extract userId from token.
3. Fetch customer using customerId + userId.
4. If customer does not belong to user:
   return CUSTOMER_NOT_FOUND.

5. Use customer.entity_id.

6. Fetch parent venue by parentId from OWPROV or MDU local venue cache.

7. Validate parent venue belongs to customer.entity_id.

8. Read parent.children array.

9. Calculate:
   childCount = len(parent.children)

10. Apply pagination on parent.children array:
   start = (page - 1) * pageSize
   end = start + pageSize

11. Take child IDs for current page.

12. Fetch child venue details using selected child IDs.

13. For each child venue, calculate:
   - hasChildren = len(child.children) > 0
   - childCount = len(child.children)

14. Return paginated children.
```

### Pagination Formula

```text
page = 1
pageSize = 50

start = (page - 1) * pageSize
end = start + pageSize
```

Example:

```text
children = [ven1, ven2, ven3, ven4, ven5]

page = 1
pageSize = 2

result IDs = [ven1, ven2]
```

For page 2:

```text
start = (2 - 1) * 2 = 2
end = 2 + 2 = 4

result IDs = [ven3, ven4]
```

### Example Response

```json
{
  "data": [
    {
      "venueId": "ven_tower_a_01J",
      "friendlyName": "Tower A",
      "kind": "tower",
      "hasChildren": true,
      "childCount": 12
    },
    {
      "venueId": "ven_tower_b_01J",
      "friendlyName": "Tower B",
      "kind": "tower",
      "hasChildren": true,
      "childCount": 10
    }
  ],
  "meta": {
    "customerId": "cus_01J...",
    "entityId": "849242cf-466a-449c-bece-08b8b0980f80",
    "parentId": "ven_root_01J",
    "page": 1,
    "pageSize": 50,
    "totalItems": 2,
    "totalPages": 1,
    "hasNextPage": false
  }
}
```

---

## 3. UI Pattern

The UI should use lazy loading.

```text
1. UI calls:
   GET /api/v1/mdu/customers/{customerId}/venues

2. MDU returns root venue data.

3. User expands a root venue.

4. UI calls:
   GET /api/v1/mdu/customers/{customerId}/venues/{parentId}/children?page=1&pageSize=50

5. MDU returns children of that parent.

6. User expands child venue.

7. UI calls same children API with new parentId.
```

This avoids loading the full recursive tree in one response.

---

## 4. OWPROV Usage

### Get Customer Entity

MDU can fetch the customer's OWPROV entity using:

```http
GET /api/v1/entity/{entityId}
```

The entity response may contain:

```json
{
  "id": "849242cf-466a-449c-bece-08b8b0980f80",
  "venues": [
    "ven_root_01J"
  ]
}
```

### Get Parent Venue

MDU should fetch parent venue details from OWPROV or from the MDU local venue cache.

The parent venue should contain:

```json
{
  "id": "ven_root_01J",
  "entity": "849242cf-466a-449c-bece-08b8b0980f80",
  "children": [
    "ven_tower_a_01J",
    "ven_tower_b_01J"
  ]
}
```

The children count comes from:

```text
len(parent.children)
```

---

## 5. Why Only Two APIs

Using only two APIs keeps UI and backend simple.

```http
GET /customers/{customerId}/venues
```

is used for root/customer-level data.

```http
GET /customers/{customerId}/venues/{parentId}/children
```

is used for expanding any node.

This avoids APIs like:

```http
GET /venues?scope=tree
GET /venues?scope=flat
GET /venues?scope=children
GET /venues/{venueId}?scope=tree
```

and gives a cleaner contract.

---

## 6. Avoid This Pattern

Do not paginate inside every nested `children` array.

Avoid:

```json
{
  "venueId": "ven_root_01J",
  "children": {
    "data": [],
    "page": 1,
    "pageSize": 50
  }
}
```

This becomes hard for UI and backend to manage.

Use this instead:

```text
Root API:
  GET /customers/{customerId}/venues

Children API:
  GET /customers/{customerId}/venues/{parentId}/children?page=1&pageSize=50
```

---

## 7. Final Pagination Rule

For recursive venue data:

```text
Do not return full unlimited recursive data.

Return root-level data by customerId.

Return paginated children by parentId.

Calculate childCount from the venue.children array.

Fetch child details only for the current page.
```

---

# 20. Final Rules

```text
1. MDU validates owsec token for every API.
2. MDU extracts userId and enforces customer ownership.
3. Customers are always created with created_by_user_id.
4. MDU calls OWPROV for entity/venue operations.
5. MDU is the Saga orchestrator.
6. Every create/update/delete/import operation has Saga steps.
7. Every external change that may need rollback gets a compensation record.
8. Failed compensation is retried by background worker with backoff.
9. Recursive data must be paginated through parent/children lazy loading.
10. Full recursive tree responses must be limited by maxDepth/maxNodes.
```

---

# MDU Device, Venue Device Assignment, and Configuration Wrapper API Flow

## 1. Purpose

This document updates the MDU API flow for device inventory, venue-device assignment, device import to venue, and configuration operations.

The frontend will call MDU wrapper APIs. MDU will internally call OWPROV APIs.

For any venue-scoped API, MDU must verify that the venue belongs to an entity owned by the logged-in user.

Flow:
1. Validate token.
2. Extract userId.
3. Resolve venueId to entityId using OWPROV or local venue cache.
4. Check mdu_customers.entity_id = resolved entityId AND created_by_user_id = userId.
5. If not found, return VENUE_NOT_FOUND or CUSTOMER_NOT_FOUND.

If CSV import partially succeeds and MDU does not store import batch state, MDU cannot automatically recover after process crash.
Therefore, the response must include successful serialNumbers and failed rows.
Frontend/operator must use DELETE /devices/import for manual cleanup if needed.

OWPROV source APIs used by this document:

```text
Inventory:
GET    /api/v1/inventory
GET    /api/v1/inventory/{serialNumber}
POST   /api/v1/inventory/{serialNumber}
PUT    /api/v1/inventory/{serialNumber}
DELETE /api/v1/inventory/{serialNumber}

Venue:
GET    /api/v1/venue
GET    /api/v1/venue/{uuid}
POST   /api/v1/venue/{uuid}
PUT    /api/v1/venue/{uuid}
DELETE /api/v1/venue/{uuid}

Configuration:
GET    /api/v1/configuration
GET    /api/v1/configuration/{uuid}
POST   /api/v1/configuration/{uuid}
PUT    /api/v1/configuration/{uuid}
DELETE /api/v1/configuration/{uuid}
```

Important OWPROV behavior:

```text
There is no separate /venue/{uuid}/devices create/update/delete endpoint.
Device membership in a venue is controlled by the inventory record using the `venue` field.
```

---

## 2. Saga Decision

For the APIs in this document, Saga is **not required** if MDU does not save local state.

Reason:

```text
MDU acts as a wrapper/proxy.
Only OWPROV state changes.
MDU does not commit local DB state for devices/configurations.
There is no distributed transaction between MDU DB and OWPROV DB.
```

Use Saga only if MDU later starts storing local records such as:

```text
device import batches
device-to-venue mappings
configuration metadata
assignment history
retryable import status
```

Even without Saga, MDU should expose reverse APIs:

```text
create/import -> delete
assign -> unassign
update -> restore old value if old value is available
delete -> usually no compensation unless old snapshot exists
```

---

## 3. Common MDU Wrapper Rules

Every MDU wrapper API must:

```text
1. Validate owsec token.
2. Extract userId and roles.
3. Validate customer/venue ownership if customer-scoped.
4. Validate request body or CSV.
5. Call OWPROV API.
6. Normalize OWPROV response.
7. Return created/updated IDs so frontend/operator can manually reverse changes if needed.
```

---

## 4. Device Inventory APIs

### 4.1 Create Device in Inventory

#### MDU API

```http
POST /api/v1/mdu/devices
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

#### Request Body

```json
{
  "serialNumber": "aabbccddeeff",
  "name": "AP-01",
  "deviceType": "AP",
  "devClass": "any"
}
```

Create and directly assign to venue:

```json
{
  "serialNumber": "aabbccddeeff",
  "name": "AP-01",
  "deviceType": "AP",
  "venue": "venue-uuid"
}
```

Create and assign device-specific configuration:

```json
{
  "serialNumber": "aabbccddeeff",
  "name": "AP-01",
  "deviceType": "AP",
  "deviceConfiguration": "configuration-uuid"
}
```

#### OWPROV API Called by MDU

```http
POST /api/v1/inventory/{serialNumber}
```

The path `serialNumber` must match body `serialNumber`.

#### Saga Required?

```text
No.
```

#### Compensating API

```http
DELETE /api/v1/mdu/devices/{serialNumber}
```

Internally calls:

```http
DELETE /api/v1/inventory/{serialNumber}
```

---

### 4.2 Update Device in Inventory

#### MDU API

```http
PUT /api/v1/mdu/devices/{serialNumber}
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

#### Request Body

```json
{
  "name": "AP-01 Updated",
  "description": "Lobby AP",
  "deviceType": "AP",
  "devClass": "any"
}
```

#### OWPROV API Called by MDU

```http
PUT /api/v1/inventory/{serialNumber}
```

#### Saga Required?

```text
No.
```

#### Manual Compensation

For update compensation, restore old state.

```text
1. GET /api/v1/mdu/devices/{serialNumber}
2. Keep old response in frontend/admin context.
3. PUT /api/v1/mdu/devices/{serialNumber} with new state.
4. To undo, PUT the old state back.
```

---

### 4.3 Get Devices From Inventory

#### MDU API

```http
GET /api/v1/mdu/devices
Authorization: Bearer <owsec-token>
```

#### Supported Query Parameters

```text
venue=<venue-uuid>
entity=<entity-uuid>
subscriber=<subscriber-uuid>
unassigned=true
subscribersOnly=true
serialOnly=true
countOnly=true
offset=<n>
limit=<n>
select=id1,id2
orderBy=serialNumber:a
rrmOnly=true
```

#### OWPROV API Called by MDU

```http
GET /api/v1/inventory
```

Examples:

```http
GET /api/v1/inventory?venue=<venue-uuid>
GET /api/v1/inventory?venue=<venue-uuid>&serialOnly=true
GET /api/v1/inventory?offset=0&limit=50
```

#### Saga Required?

```text
No. Read-only.
```

---

### 4.4 Get One Device From Inventory

#### MDU API

```http
GET /api/v1/mdu/devices/{serialNumber}
Authorization: Bearer <owsec-token>
```

#### Supported Query Parameters

```text
config=true
config=true&explain=true
resolveConfig=true
applyConfiguration=true
firmwareOptions=true
rrmSettings=true
```

#### OWPROV API Called by MDU

```http
GET /api/v1/inventory/{serialNumber}
```

Examples:

```http
GET /api/v1/inventory/{serialNumber}
GET /api/v1/inventory/{serialNumber}?resolveConfig=true
GET /api/v1/inventory/{serialNumber}?applyConfiguration=true
```

#### Saga Required?

```text
No.
```

`applyConfiguration=true` pushes config through OWPROV/gateway, but Saga is still not required unless MDU also stores local state.

---

### 4.5 Delete Device From Inventory

#### MDU API

```http
DELETE /api/v1/mdu/devices/{serialNumber}
Authorization: Bearer <owsec-token>
```

#### OWPROV API Called by MDU

```http
DELETE /api/v1/inventory/{serialNumber}
```

#### Saga Required?

```text
No.
```

#### Compensation

Usually no compensation for delete.

If restore is required, MDU must have old device data and call:

```http
POST /api/v1/mdu/devices
```

to recreate it.

---

## 5. Device Import Into Inventory

### MDU API

```http
POST /api/v1/mdu/devices/import
Authorization: Bearer <owsec-token>
Content-Type: multipart/form-data
```

### Form Data

```text
file = devices.csv
```

### CSV Example

```csv
serialNumber,name,deviceType
aabbccddeeff,AP-01,AP
aabbccddee00,AP-02,AP
```

### MDU Internal Flow

```text
1. Validate token.
2. Parse CSV.
3. Validate serialNumber, name, deviceType.
4. For each row, call OWPROV POST /inventory/{serialNumber}.
5. Collect successful and failed rows.
6. Return created devices and failed rows.
```

### OWPROV API Called by MDU

For each row:

```http
POST /api/v1/inventory/{serialNumber}
```

### Saga Required?

```text
No, if MDU does not store import batch locally.
```

### Compensating API

```http
DELETE /api/v1/mdu/devices/import
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

Request:

```json
{
  "serialNumbers": [
    "aabbccddeeff",
    "aabbccddee00"
  ]
}
```

Internally calls:

```http
DELETE /api/v1/inventory/{serialNumber}
```

for each serial number.

---

## 6. Device and Venue Assignment APIs

OWPROV device-to-venue assignment is controlled through the inventory record's `venue` field.

---

### 6.1 Add Device to Venue

#### MDU API

```http
POST /api/v1/mdu/venues/{venueId}/devices
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

#### Request Body

```json
{
  "serialNumbers": [
    "aabbccddeeff",
    "aabbccddee00"
  ]
}
```

#### OWPROV API Called by MDU

For each serial number:

```http
PUT /api/v1/inventory/{serialNumber}
```

Body:

```json
{
  "venue": "venue-uuid"
}
```

#### Saga Required?

```text
No.
```

#### Compensating API

```http
DELETE /api/v1/mdu/venues/{venueId}/devices
Content-Type: application/json
```

Request:

```json
{
  "serialNumbers": [
    "aabbccddeeff",
    "aabbccddee00"
  ]
}
```

Internally calls:

```http
PUT /api/v1/inventory/{serialNumber}
```

with:

```json
{
  "venue": ""
}
```

---

### 6.2 Delete Device From Venue

#### MDU API

```http
DELETE /api/v1/mdu/venues/{venueId}/devices/{serialNumber}
Authorization: Bearer <owsec-token>
```

#### OWPROV API Called by MDU

```http
PUT /api/v1/inventory/{serialNumber}
```

Body:

```json
{
  "venue": ""
}
```

#### Saga Required?

```text
No.
```

#### Compensation

Add it back:

```http
POST /api/v1/mdu/venues/{venueId}/devices
```

---

### 6.3 Get Devices From Venue

#### MDU API

```http
GET /api/v1/mdu/venues/{venueId}/devices
Authorization: Bearer <owsec-token>
```

#### Query Parameters

```text
offset=<n>
limit=<n>
serialOnly=true
includeChildren=true
```

#### OWPROV APIs Called by MDU

Full inventory records:

```http
GET /api/v1/inventory?venue=<venue-uuid>
```

Serial numbers only:

```http
GET /api/v1/inventory?venue=<venue-uuid>&serialOnly=true
```

Alternative venue API:

```http
GET /api/v1/venue/{uuid}?getDevices=true
```

Include child venues:

```http
GET /api/v1/venue/{uuid}?getDevices=true&getChildren=true
```

#### Saga Required?

```text
No. Read-only.
```

---

### 6.4 Update Device From Venue

This means changing a device's venue assignment or updating a device while it is associated with a venue.

#### MDU API

```http
PUT /api/v1/mdu/venues/{venueId}/devices/{serialNumber}
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

#### Request Body

```json
{
  "venue": "new-venue-uuid",
  "name": "AP-01 Updated"
}
```

#### OWPROV API Called by MDU

```http
PUT /api/v1/inventory/{serialNumber}
```

#### Saga Required?

```text
No.
```

#### Manual Compensation

Fetch old inventory state before update if rollback is needed, then restore by calling:

```http
PUT /api/v1/mdu/devices/{serialNumber}
```

---

### 6.5 Import Device in Venue

#### MDU API

```http
POST /api/v1/mdu/venues/{venueId}/devices/import
Authorization: Bearer <owsec-token>
Content-Type: multipart/form-data
```

#### Form Data

```text
file = devices.csv
```

#### CSV Example

```csv
serialNumber,name,deviceType
aabbccddeeff,AP-01,AP
aabbccddee00,AP-02,AP
```

#### MDU Internal Flow

```text
1. Validate token.
2. Validate venueId.
3. Parse CSV.
4. For each device, call OWPROV POST /inventory/{serialNumber} with venue field.
5. Return created/assigned and failed rows.
```

#### OWPROV API Called by MDU

Preferred single call per device:

```http
POST /api/v1/inventory/{serialNumber}
```

Body:

```json
{
  "serialNumber": "aabbccddeeff",
  "name": "AP-01",
  "deviceType": "AP",
  "venue": "venue-uuid"
}
```

#### Saga Required?

```text
No, if MDU does not store import state locally.
```

#### Compensating APIs

Unassign only:

```http
DELETE /api/v1/mdu/venues/{venueId}/devices
```

with:

```json
{
  "serialNumbers": [
    "aabbccddeeff"
  ]
}
```

Unassign and delete inventory:

```http
DELETE /api/v1/mdu/venues/{venueId}/devices/import
```

with:

```json
{
  "mode": "unassignAndDelete",
  "serialNumbers": [
    "aabbccddeeff"
  ]
}
```

Recommended default:

```text
unassignOnly
```

---

## 7. Configuration APIs

### 7.1 Add Configuration

#### MDU API

```http
POST /api/v1/mdu/configurations
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

#### Request Body

```json
{
  "uuid": "configuration-uuid",
  "name": "Venue AP Config",
  "deviceTypes": ["AP"],
  "venue": "venue-uuid",
  "configuration": [
    {
      "name": "globals",
      "weight": 0,
      "configuration": "{\"globals\":{\"hostname\":\"ap-template\"}}"
    }
  ]
}
```

#### OWPROV API Called by MDU

```http
POST /api/v1/configuration/{uuid}
```

#### Saga Required?

```text
No.
```

#### Compensating API

```http
DELETE /api/v1/mdu/configurations/{configurationId}
```

Internally calls:

```http
DELETE /api/v1/configuration/{uuid}
```

---

### 7.2 Delete Configuration

#### MDU API

```http
DELETE /api/v1/mdu/configurations/{configurationId}
Authorization: Bearer <owsec-token>
```

#### OWPROV API Called by MDU

```http
DELETE /api/v1/configuration/{uuid}
```

#### Important OWPROV Behavior

Configuration cannot be deleted while its `inUse` list is non-empty.

Remove device, venue, entity, policy, or variable references first.

#### Saga Required?

```text
No.
```

---

### 7.3 Update Configuration

#### MDU API

```http
PUT /api/v1/mdu/configurations/{configurationId}
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

#### OWPROV API Called by MDU

```http
PUT /api/v1/configuration/{uuid}
```

#### Saga Required?

```text
No.
```

#### Manual Compensation

To undo update, restore old configuration:

```text
1. GET /api/v1/mdu/configurations/{configurationId}
2. Store old response on client/admin side if needed.
3. PUT /api/v1/mdu/configurations/{configurationId} with new data.
4. To undo, PUT old data back.
```

---

### 7.4 Get Configuration

#### MDU APIs

```http
GET /api/v1/mdu/configurations
GET /api/v1/mdu/configurations/{configurationId}
```

#### Supported Query Parameters

```text
venue=<venue-uuid>
entity=<entity-uuid>
select=id1,id2
countOnly=true
offset=<n>
limit=<n>
expandInUse=true
computedAffected=true
```

#### OWPROV APIs Called by MDU

```http
GET /api/v1/configuration
GET /api/v1/configuration/{uuid}
```

Examples:

```http
GET /api/v1/configuration?venue=<venue-uuid>
GET /api/v1/configuration?entity=<entity-uuid>
GET /api/v1/configuration/{uuid}?computedAffected=true
```

#### Saga Required?

```text
No. Read-only.
```

---

## 8. Update Configuration of Venue

Configuration assignment to a venue is done through the configuration record's `venue` field.

### MDU API

```http
PUT /api/v1/mdu/venues/{venueId}/configuration/{configurationId}
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

### Request Body

```json
{
  "deviceTypes": ["AP"],
  "configuration": []
}
```

### OWPROV API Called by MDU

```http
PUT /api/v1/configuration/{uuid}
```

with:

```json
{
  "venue": "venue-uuid",
  "deviceTypes": ["AP"],
  "configuration": []
}
```

### Saga Required?

```text
No.
```

### Compensating APIs

Remove venue assignment:

```http
DELETE /api/v1/mdu/venues/{venueId}/configuration/{configurationId}
```

Internally calls:

```http
PUT /api/v1/configuration/{uuid}
```

with:

```json
{
  "venue": ""
}
```

Restore old config if known:

```http
PUT /api/v1/mdu/configurations/{configurationId}
```

---

## 9. Update Configuration of Access Point

Device-specific configuration is assigned using the inventory record field `deviceConfiguration`.

### MDU API

```http
PUT /api/v1/mdu/access-points/{serialNumber}/configuration/{configurationId}
Authorization: Bearer <owsec-token>
Content-Type: application/json
```

### OWPROV API Called by MDU

```http
PUT /api/v1/inventory/{serialNumber}
```

with:

```json
{
  "deviceConfiguration": "configuration-uuid"
}
```

### Saga Required?

```text
No.
```

### Compensating API

Remove device-specific config:

```http
DELETE /api/v1/mdu/access-points/{serialNumber}/configuration
```

Internally calls:

```http
PUT /api/v1/inventory/{serialNumber}
```

with:

```json
{
  "deviceConfiguration": ""
}
```

Restore old config if known:

```http
PUT /api/v1/mdu/access-points/{serialNumber}/configuration/{oldConfigurationId}
```

---

### 9.1 Get Effective AP Configuration

#### MDU API

```http
GET /api/v1/mdu/access-points/{serialNumber}/configuration
Authorization: Bearer <owsec-token>
```

#### OWPROV API Called by MDU

Resolve config without pushing:

```http
GET /api/v1/inventory/{serialNumber}?resolveConfig=true
```

Compute and push config:

```http
POST /api/v1/mdu/access-points/{serialNumber}/configuration/apply
```

MDU exposes apply as POST because it causes a side effect, even though OWPROV internally uses GET with applyConfiguration=true.

Internally calls:

```http
GET /api/v1/inventory/{serialNumber}?applyConfiguration=true
```

#### Saga Required?

```text
No.
```

---

## 10. Full MDU Wrapper API List

### 10.1 Inventory Device APIs

```http
GET    /api/v1/mdu/devices
GET    /api/v1/mdu/devices/{serialNumber}
POST   /api/v1/mdu/devices
PUT    /api/v1/mdu/devices/{serialNumber}
DELETE /api/v1/mdu/devices/{serialNumber}
POST   /api/v1/mdu/devices/import
DELETE /api/v1/mdu/devices/import
```

### 10.2 Venue Device APIs

```http
GET    /api/v1/mdu/venues/{venueId}/devices
POST   /api/v1/mdu/venues/{venueId}/devices
PUT    /api/v1/mdu/venues/{venueId}/devices/{serialNumber}
DELETE /api/v1/mdu/venues/{venueId}/devices/{serialNumber}
DELETE /api/v1/mdu/venues/{venueId}/devices
POST   /api/v1/mdu/venues/{venueId}/devices/import
DELETE /api/v1/mdu/venues/{venueId}/devices/import
```

### 10.3 Configuration APIs

```http
GET    /api/v1/mdu/configurations
GET    /api/v1/mdu/configurations/{configurationId}
POST   /api/v1/mdu/configurations
PUT    /api/v1/mdu/configurations/{configurationId}
DELETE /api/v1/mdu/configurations/{configurationId}
```

### 10.4 Venue Configuration APIs

```http
GET    /api/v1/mdu/venues/{venueId}/configurations
PUT    /api/v1/mdu/venues/{venueId}/configuration/{configurationId}
DELETE /api/v1/mdu/venues/{venueId}/configuration/{configurationId}
```

### 10.5 Access Point Configuration APIs

```http
GET    /api/v1/mdu/access-points/{serialNumber}/configuration
PUT    /api/v1/mdu/access-points/{serialNumber}/configuration/{configurationId}
DELETE /api/v1/mdu/access-points/{serialNumber}/configuration
POST   /api/v1/mdu/access-points/{serialNumber}/configuration/apply
```

---

## 11. Wrapper to OWPROV Mapping Summary

| MDU Wrapper API | OWPROV API |
|---|---|
| `GET /mdu/devices` | `GET /inventory` |
| `GET /mdu/devices/{serialNumber}` | `GET /inventory/{serialNumber}` |
| `POST /mdu/devices` | `POST /inventory/{serialNumber}` |
| `PUT /mdu/devices/{serialNumber}` | `PUT /inventory/{serialNumber}` |
| `DELETE /mdu/devices/{serialNumber}` | `DELETE /inventory/{serialNumber}` |
| `POST /mdu/devices/import` | loop over `POST /inventory/{serialNumber}` |
| `DELETE /mdu/devices/import` | loop over `DELETE /inventory/{serialNumber}` |
| `GET /mdu/venues/{venueId}/devices` | `GET /inventory?venue={venueId}` or `GET /venue/{uuid}?getDevices=true` |
| `POST /mdu/venues/{venueId}/devices` | loop over `PUT /inventory/{serialNumber}` with `venue` |
| `PUT /mdu/venues/{venueId}/devices/{serialNumber}` | `PUT /inventory/{serialNumber}` |
| `DELETE /mdu/venues/{venueId}/devices/{serialNumber}` | `PUT /inventory/{serialNumber}` with `venue: ""` |
| `POST /mdu/venues/{venueId}/devices/import` | loop over `POST /inventory/{serialNumber}` with `venue` |
| `GET /mdu/configurations` | `GET /configuration` |
| `GET /mdu/configurations/{configurationId}` | `GET /configuration/{uuid}` |
| `POST /mdu/configurations` | `POST /configuration/{uuid}` |
| `PUT /mdu/configurations/{configurationId}` | `PUT /configuration/{uuid}` |
| `DELETE /mdu/configurations/{configurationId}` | `DELETE /configuration/{uuid}` |
| `PUT /mdu/venues/{venueId}/configuration/{configurationId}` | `PUT /configuration/{uuid}` with `venue` |
| `GET /mdu/venues/{venueId}/configurations` | `GET /configuration?venue={venueId}` |
| `PUT /mdu/access-points/{serialNumber}/configuration/{configurationId}` | `PUT /inventory/{serialNumber}` with `deviceConfiguration` |
| `GET /mdu/access-points/{serialNumber}/configuration` | `GET /inventory/{serialNumber}?resolveConfig=true` |
| `POST /mdu/access-points/{serialNumber}/configuration/apply` | `GET /inventory/{serialNumber}?applyConfiguration=true` |

---

## 12. Saga Decision Summary For Device Add and configuration

| Operation | Saga Required? | Reason |
|---|---|---|
| Create device in inventory | No | Only OWPROV state changes |
| Update device in inventory | No | Only OWPROV state changes |
| Get devices from inventory | No | Read-only |
| Delete device from inventory | No | Only OWPROV state changes |
| Get one device from inventory | No | Read-only |
| Add device to venue | No | Inventory record updated in OWPROV |
| Delete device from venue | No | Inventory record updated in OWPROV |
| Get device from venue | No | Read-only |
| Update device from venue | No | Inventory record updated in OWPROV |
| Import device in venue | No, unless MDU stores import batch | Only OWPROV state changes |
| Add configuration | No | Only OWPROV state changes |
| Delete configuration | No | Only OWPROV state changes |
| Update configuration | No | Only OWPROV state changes |
| Get configuration | No | Read-only |
| Update configuration of venue | No | Only OWPROV config state changes |
| Update configuration of AP | No | Only OWPROV inventory/config state changes |

If later MDU stores imported devices, assignment records, or config metadata locally, then use Saga for those multi-system writes.
