# Impact Analysis: Hierarchical Constraints and Fixed Policy Design

This document details the modifications required in the `mango-mdu-service` codebase to align with the new hierarchical constraints and fixed policy model from the provisioning service.

---

## 1. Summary of Architectural Changes

| Core Concept | Old Model | New Model |
| :--- | :--- | :--- |
| **Entity Hierarchy** | Nested Customer/Normal entities were permitted. | Operator Entities exist under Root. Customer Entities exist under Root or Operator Entities. **No nested Customer Entities (normal/subscriber) are allowed.** |
| **Policies** | Generated dynamically per user/role assignment. Contain inline JSON scope limits. | Pre-defined, shared, and **immutable system policies** (Admin, CSR, NOC, Installer). |
| **Role Assignment** | Role points to user-specific custom policy. | Role points to a fixed system policy; the target scope is defined entirely by the role's attributes. |
| **Access Mutation** | Mutates the custom policy's resource permissions list. | Updates the role's policy template reference (e.g. `CSR` -> `Admin`). |

---

## 2. Required Code Changes in MDU

### A. Entity Creation Validation
* **File:** [entity_service.go](file:///home/iotina/routerarchitects_repos/mango-mdu-service/internal/services/entity_service.go)
* **Function:** `CreateEntity`
* **Changes:**
  * When a child entity is being created (`ParentEntityID != ""` and `ParentEntityID != "00000000-0000-0000-0000-000000000000"`):
    * Retrieve the parent entity details from the provisioning client using `s.provClient.GetEntity`.
    * If the parent entity's `Type` is `normal` or `subscriber` (Customer entity types), reject the creation with:
      ```go
      apperror.New(apperror.CodeInvalidInput, "cannot create a child entity under a customer entity")
      ```

### B. Assignment Service Refactoring
* **File:** [assignment_service.go](file:///home/iotina/routerarchitects_repos/mango-mdu-service/internal/services/assignment_service.go)
* **Function:** `CreateAssignment`
  * Remove policy listing and matching checks for custom policies (`req.Role + "Policy-" + userID`).
  * Remove dynamic policy creation logic (`provClient.CreatePolicy`).
  * Map `req.Role` to the correct fixed system policy ID (e.g., `ADMIN_POLICY_ID`, `CSR_POLICY_ID`, `NOC_POLICY_ID`, `INSTALLER_POLICY_ID`).
  * Create the `ProvManagementRole` pointing to the target entity/venue and referencing the mapped fixed policy ID.
* **Function:** `DeleteAssignment`
  * Remove the call to `s.provClient.DeletePolicy`, as system policies are immutable and shared across all assignments.
* **Function:** `UpdateAccessPolicy`
  * This function and its corresponding HTTP endpoint (`PUT /access-policy`) are deprecated/removed, since normal users do not customize individual permissions. Only Root will create/manage policies via downstream/private REST APIs.


---

## 3. Required Test Suite Changes

### A. Granular & Risky Tests
* **File:** [risky_test.go](file:///home/iotina/routerarchitects_repos/mango-mdu-service/internal/http/handlers/risky_test.go)
* **Changes:**
  * Deprecate/remove:
    * `TestRiskyBehaviors/Policy created but role creation fails` (since policies are no longer created on assignment).
    * `TestRiskyBehaviors/Role deletion succeeds but policy deletion fails` (since policies are no longer deleted).
  * Refactor:
    * `TestRiskyBehaviors/Access policy update cannot modify another user's policy` to verify that mutating the policy template reference on the role is blocked if the caller lacks authorization.
  * Mock Server:
    * Seed the 4 fixed system policies (`ADMIN_POLICY_ID`, etc.) in the mock server instead of accepting dynamic policy creation/deletion POST/DELETE requests.

---

## 4. Resolved Design Decisions

> [!NOTE]
> **1. Policy ID Configuration (Resolved)**
> When creating or updating an assignment, MDU will first fetch all system policies from the provisioning client using `provClient.ListAllPolicies(reqCtx)`. It will then locate the matching policy whose name matches the requested role template (e.g. "admin", "csr", "noc", "installer"), rather than hardcoding or managing policy UUIDs locally.
>
> **2. Handling `UpdateAccessPolicy` (Resolved)**
> The `PUT /access-policy` endpoint and `UpdateAccessPolicy` service method will be removed entirely, as users will only select from existing fixed system policies.


