// Package ratelimit owns per-API-key rate limiting (T0005): the algorithm (token bucket vs fixed window is the T0005
// tower_question) and the Retry-After computation.
//
// Package boundary:
//   - owns: the limiter and its clock injection.
//   - does not own: where the limit value lives (tenant.rate_limit),
//     returning 429 (httpapi).
package ratelimit
