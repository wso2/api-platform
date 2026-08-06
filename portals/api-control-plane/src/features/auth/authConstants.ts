// The BFF checks for this header on every state-mutating request (GET/HEAD/OPTIONS
// are exempt) — a fixed contract between the BFF and this SPA, not something either
// side can vary independently. Must match api-control-plane-bff's CSRFHeaderName.
export const CSRF_HEADER = 'X-Requested-By';
export const CSRF_HEADER_VALUE = 'api-control-plane';
