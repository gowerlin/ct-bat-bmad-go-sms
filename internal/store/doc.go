// Package store owns persistence of the entities in the docs/README.md data
// model (Tenant, Message, Template, DeliveryReport, OutboundNotification).
// The course uses an in-memory implementation; the SPINE keeps this package
// as the database boundary so a real DB can replace it without touching
// callers.
//
// Package boundary:
//   - owns: entity structs, the repository interfaces, the in-memory
//     implementation, status filtering for GET /v1/messages?status=.
//   - does not own: validation (template / tenant), HTTP (httpapi), sending.
package store
