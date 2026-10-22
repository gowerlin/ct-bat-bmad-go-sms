// Package webhook owns both directions of delivery status
// (COURSE-PLAN § 5.1 #4, T0009): inbound supplier callbacks decoded by the
// Provider and applied to the message, and outbound signed notifications to
// the tenant's callback_url (HMAC, delivered exactly once).
//
// Package boundary:
//   - owns: ProviderRef → message mapping, status update, the
//     OutboundNotification entity (§ 5.3) and its signature / idempotency.
//   - does not own: decoding the supplier payload (provider), persistence
//     (store), the HTTP route (httpapi).
package webhook
