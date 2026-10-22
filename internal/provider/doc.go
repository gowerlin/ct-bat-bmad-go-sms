// Package provider defines the SMS supplier boundary: the Provider interface
// and the message / receipt / delivery-report types that cross it
// (COURSE-PLAN § 6.2).
//
// Package boundary:
//   - owns: the adapter contract. Sub-packages implement it: mock (course
//     mainline, fault injection from T0002) and mitake (CH5 / T0010, a
//     simulated Mitake-style HTTP adapter: form POST send, form callback
//     with a shared secret; tested only against httptest.Server, fake
//     credentials only). providertest is the shared contract both pass
//     (AD-1). The assembly root picks one with PROVIDER=mock|mitake.
//   - does not own: retries (queue), status persistence (store), callback
//     routing and tenant notification (webhook).
package provider
