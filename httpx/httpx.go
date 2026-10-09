// Package httpx binds net/http requests to a business: the middleware derives the param from the request,
// resolves it and passes a bound context to the next handler, so that easyext.First / easyext.All work
// anywhere below it.
//
// Wrap only the routes that use extension points; health checks and the like then need no business identity.
package httpx

import (
	"errors"
	"net/http"

	"github.com/xiaoshicae/go-easy-extension/v2"
)

// ErrorHandler writes the response when a request cannot be bound.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// Option configures [Middleware].
type Option func(*options)

type options struct {
	onError ErrorHandler
}

// OnError replaces the default error handling.
func OnError(h ErrorHandler) Option {
	return func(o *options) { o.onError = h }
}

// Middleware binds every request handled by next. param derives the matcher param from the request, e.g. from
// a header, the path or the authenticated user; it should not consume the body.
//
// By default a param error answers 400 Bad Request and a resolution error (no business, several businesses,
// unknown business) answers 422 Unprocessable Entity, with the status text only: the details, which may name
// businesses, are left to [OnError].
func Middleware[T any](c *easyext.Context[T], param func(*http.Request) (T, error), opts ...Option) func(http.Handler) http.Handler {
	o := options{onError: defaultOnError}
	for _, opt := range opts {
		opt(&o)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := param(r)
			if err != nil {
				o.onError(w, r, &ParamError{Err: err})
				return
			}
			ctx, err := c.Bind(r.Context(), p)
			if err != nil {
				o.onError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ParamError wraps an error returned by the param function of [Middleware].
type ParamError struct {
	Err error
}

func (e *ParamError) Error() string { return "httpx: deriving the matcher param: " + e.Err.Error() }

func (e *ParamError) Unwrap() error { return e.Err }

func defaultOnError(w http.ResponseWriter, _ *http.Request, err error) {
	status := http.StatusInternalServerError
	if _, ok := errors.AsType[*ParamError](err); ok {
		status = http.StatusBadRequest
	} else if _, ok := errors.AsType[*easyext.ResolutionError](err); ok {
		status = http.StatusUnprocessableEntity
	}
	http.Error(w, http.StatusText(status), status)
}
