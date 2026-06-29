// Package middleware provides composable net/http middleware following the
// standard func(http.Handler) http.Handler pattern.
package middleware

import "net/http"

// Chain composes middleware left-to-right so that the first argument is the
// outermost wrapper (executed first on a request, last on a response).
//
//	Chain(A, B, C)(handler) == A(B(C(handler)))
func Chain(mw ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mw) - 1; i >= 0; i-- {
			h = mw[i](h)
		}
		return h
	}
}
