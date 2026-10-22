// Package tenant owns multi-tenancy: the Tenant entity (§ 5.3), API-key
// hashing and lookup, and tenant isolation rules (COURSE-PLAN § 5.1 #7,
// T0003).
//
// Package boundary:
//   - owns: resolving X-API-Key → tenant, the "not found across tenants"
//     rule that httpapi maps to 404.
//   - does not own: the HTTP header itself (httpapi), persistence (store).
package tenant
