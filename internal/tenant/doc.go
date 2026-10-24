// Package tenant owns multi-tenancy: the Tenant entity (data model in
// docs/README.md), API-key hashing and lookup, and tenant isolation rules
// (T0003).
//
// Package boundary:
//   - owns: resolving X-API-Key → tenant, the "not found across tenants"
//     rule that httpapi maps to 404.
//   - does not own: the HTTP header itself (httpapi), persistence (store).
package tenant
