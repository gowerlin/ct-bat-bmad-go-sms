// Package webhook owns both directions of delivery status
// (COURSE-PLAN § 5.1 #4, T0009): inbound supplier callbacks decoded by the
// Provider and applied to the message, and outbound signed notifications to
// the tenant's callback_url (HMAC, delivered exactly once).
//
// Inbound runs ParseCallback, maps ProviderRef to the message and moves it
// sent → delivered / undelivered with the outbox entry in the same store
// operation, so a repeated callback is never notified twice (AD-3, AD-7).
// Dispatcher is the single goroutine that claims outbox entries in order and
// posts {"id","status"} signed with X-Signature: sha256=<hex> (D013),
// retrying 1s, 2s, 4s through an injected wait (AD-13).
//
// Package boundary:
//   - owns: ProviderRef → message mapping, status update, the
//     OutboundNotification entity (§ 5.3) and its signature / idempotency.
//   - does not own: decoding the supplier payload (provider), persistence
//     (store), the HTTP route (httpapi), the http.Client (cmd/smsd, AD-9).
package webhook
