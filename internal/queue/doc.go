// Package queue owns the outbound send queue: ordering, worker dispatch,
// retry policy and the queued → sending → sent / failed transitions
// (T0008).
//
// Package boundary:
//   - owns: when a message is handed to the Provider, how many attempts are
//     made, back-off between attempts, marking a message failed when retries
//     are exhausted.
//   - does not own: the Provider itself (provider), the message record
//     (store), HTTP (httpapi). Back-off waits go through an injected Wait so
//     E2E tests can drive them with a fake clock; timestamps come from the
//     store's own Now.
package queue
