// Package provider defines the SMS supplier boundary: the Provider interface
// and the message / receipt / delivery-report types that cross it
// (COURSE-PLAN § 6.2).
//
// Package boundary:
//   - owns: the adapter contract. Sub-packages implement it: mock (course
//     mainline, fault injection from T0002) and mitake (CH5, Mitake-style
//     HTTP adapter against an httptest.Server; no real credentials ever).
//   - does not own: retries (queue), status persistence (store), callback
//     routing and tenant notification (webhook).
package provider
