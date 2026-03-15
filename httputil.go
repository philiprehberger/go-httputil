// Package httputil provides composable HTTP client middleware for Go.
//
// Build instrumented [http.Client] instances by stacking middleware on top of
// a base [http.RoundTripper]. Middleware is applied in order: the first
// middleware added wraps outermost and executes first.
package httputil

import "net/http"

// RoundTripperFunc is an adapter that allows the use of ordinary functions as
// [http.RoundTripper] implementations.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip calls f(r).
func (f RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Middleware wraps an [http.RoundTripper] with additional behaviour.
type Middleware func(http.RoundTripper) http.RoundTripper

// ClientOption configures an [http.Client] created by [NewClient].
type ClientOption func(*clientConfig)

type clientConfig struct {
	base        http.RoundTripper
	middlewares []Middleware
}

// WithMiddleware appends a [Middleware] to the chain.
// Middlewares are applied in order: the first middleware added wraps outermost
// and executes first on each request.
func WithMiddleware(m Middleware) ClientOption {
	return func(c *clientConfig) {
		c.middlewares = append(c.middlewares, m)
	}
}

// WithBaseTransport sets the base [http.RoundTripper] that sits at the bottom
// of the middleware chain. If not set, [http.DefaultTransport] is used.
func WithBaseTransport(rt http.RoundTripper) ClientOption {
	return func(c *clientConfig) {
		c.base = rt
	}
}

// NewClient creates an [http.Client] whose transport is a middleware chain
// built from the provided options. The returned client is a standard
// [http.Client] so callers do not need to change their code.
func NewClient(opts ...ClientOption) *http.Client {
	cfg := &clientConfig{
		base: http.DefaultTransport,
	}
	for _, o := range opts {
		o(cfg)
	}

	// Build the chain: last middleware wraps innermost (closest to transport).
	var rt http.RoundTripper = cfg.base
	for i := len(cfg.middlewares) - 1; i >= 0; i-- {
		rt = cfg.middlewares[i](rt)
	}

	return &http.Client{Transport: rt}
}
