// Package template owns message templates: CRUD per tenant and rendering of
// {{var}} placeholders (COURSE-PLAN § 5.1 #5, T0004 / T0007).
//
// Package boundary:
//   - owns: the Template entity (§ 5.3), placeholder syntax, the
//     missing-variable error that httpapi maps to 422.
//   - does not own: persistence (store), HTTP shapes (httpapi), sending.
package template
