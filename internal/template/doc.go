// Package template owns message templates: CRUD per tenant and rendering of
// {{var}} placeholders (T0004 / T0007).
//
// Package boundary:
//   - owns: the Template entity (data model in docs/README.md), placeholder
//     syntax, the missing-variable error that httpapi maps to 422.
//   - does not own: persistence (store), HTTP shapes (httpapi), sending.
package template
