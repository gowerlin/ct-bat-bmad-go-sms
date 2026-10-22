// Package queue owns the outbound send queue: ordering, worker dispatch,
// retry policy and the queued → sending → sent / failed transitions
// (COURSE-PLAN § 5.1 #3, T0008).
//
// Package boundary:
//   - owns: when a message is handed to the Provider, how many attempts are
//     made, back-off between attempts, marking a message failed when retries
//     are exhausted.
//   - does not own: the Provider itself (provider), the message record
//     (store), HTTP (httpapi). Time comes in through an injectable clock so
//     E2E tests can advance it.
package queue
