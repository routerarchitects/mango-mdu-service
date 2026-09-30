# Direct-Callable / Pass-Through APIs (PROV Delegation)

To maintain a clean architectural boundary and avoid redundant proxying, several management APIs listed in the Phase 1 specifications are not implemented in `mango-mdu-service`. Instead, client applications (such as the Mango UI) call these endpoints directly on the Provisioning service (`owprov` / PROV) or the Security service (`owsec` / SEC) using the same user token context.

Below is the mapping of these direct-callable/passthrough APIs:

---

## 1. Management Roles (`/roles`)
These endpoints are routed directly to PROV's management role endpoints:

| Northbound API Path | HTTP Method | Downstream PROV API Path | Description |
| :--- | :---: | :--- | :--- |
| `/api/v1/roles` | `GET` | `/api/v1/managementRole` | List management roles |
| `/api/v1/roles/{roleId}` | `GET` | `/api/v1/managementRole/{id}` | Retrieve details of a specific role |
| `/api/v1/roles` | `POST` | `/api/v1/managementRole/{id}` | Create a new management role |
| `/api/v1/roles/{roleId}` | `PUT` | `/api/v1/managementRole/{id}` | Update management role details |
| `/api/v1/roles/{roleId}` | `DELETE` | `/api/v1/managementRole/{id}` | Delete management role |

---

## 2. Management Policies (`/policies`)
These endpoints are routed directly to PROV's management policy endpoints:

| Northbound API Path | HTTP Method | Downstream PROV API Path | Description |
| :--- | :---: | :--- | :--- |
| `/api/v1/policies` | `GET` | `/api/v1/managementPolicy` | List management policies |
| `/api/v1/policies/{policyId}` | `GET` | `/api/v1/managementPolicy/{id}` | Retrieve details of a specific policy |
| `/api/v1/policies` | `POST` | `/api/v1/managementPolicy/{id}` | Create a new management policy |
| `/api/v1/policies/{policyId}` | `PUT` | `/api/v1/managementPolicy/{id}` | Update management policy details |
| `/api/v1/policies/{policyId}` | `DELETE` | `/api/v1/managementPolicy/{id}` | Delete management policy |

---

## 3. Operator Member Management (`/operators`)
These endpoints manage operator member details directly on PROV:

| Northbound API Path | HTTP Method | Downstream PROV API Path | Description |
| :--- | :---: | :--- | :--- |
| `/api/v1/operators/{operatorId}` | `GET` | `/api/v1/operator/{id}` | Retrieve details of a specific operator |
| `/api/v1/operators/{operatorId}` | `PUT` | `/api/v1/operator/{id}` | Update operator details |
| `/api/v1/operators/{operatorId}` | `DELETE` | `/api/v1/operator/{id}` | Delete operator |

*Note: Collection-level operator operations (listing and creating operators) also bypass MDU and call PROV directly.*

---

## 4. Subscriber/Signup List (`/subscribers`)
Subscriber signup listing is retrieved directly from PROV's signup endpoint:

| Northbound API Path | HTTP Method | Downstream PROV API Path | Description |
| :--- | :---: | :--- | :--- |
| `/api/v1/operators/{operatorId}/subscribers` | `GET` | `/api/v1/signup` | Retrieve subscriber signup list for an operator |
