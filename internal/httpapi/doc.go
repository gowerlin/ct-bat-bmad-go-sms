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
// COURSE-PLAN § 7.1 names this directory `api`; § 4 (1-06) names it
// `httpapi`. The skeleton uses `httpapi` because the name says what kind of
// API it is (HTTP), leaving `api` free for a future non-HTTP surface.
package httpapi
