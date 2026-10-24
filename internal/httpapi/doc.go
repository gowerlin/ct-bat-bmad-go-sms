// Package httpapi is the HTTP boundary of the SMS platform: routing, request
// decoding, response encoding and the error envelope
// {"error":{"code":"…","message":"…"}}.
//
// Package boundary:
//   - owns: the http.Handler, route table (docs/openapi.yaml), status codes,
//     JSON shapes, API-key extraction from the X-API-Key header.
//   - does not own: business rules. Sending goes through queue, tenants through
//     tenant, templates through template, limits through ratelimit, persistence
//     through store, provider callbacks through webhook.
//
// Adding a feature (D004: explicit stubs, no init() registry):
//   - routes: fill the feature's registerXxx (messages_list.go, templates.go);
//     NewHandler already calls it. Wrap every /v1 route in s.v1(...).
//   - middleware order is fixed in v1: authenticate, rateLimit, handler (AD-10).
//     /healthz is not wrapped.
//   - errors: fill templateError / rateLimitError to return an *apiError; they
//     are consulted via featureErrorMappers before the 500 default, and
//     writeError remains the only place the envelope is written (AD-8).
//
// The directory is `httpapi`, not `api`, because the name says what kind of
// API it is (HTTP), leaving `api` free for a future non-HTTP surface.
package httpapi
