package httputil

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// contextKey is an unexported type for context keys defined in this package.
type contextKey int

// ContextKey is the type used for context keys in this package.
type ContextKey = contextKey

const (
	// requestIDKey is the context key for the request ID value.
	requestIDKey contextKey = iota
)

// WithRequestIDValue returns a copy of ctx with the request ID value set.
// Use this to attach a request ID to the context before making an HTTP call
// with a client configured via [WithRequestID].
func WithRequestIDValue(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext extracts the request ID from ctx. It returns an empty
// string if no request ID is present.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithBearerToken returns a [ClientOption] that injects an Authorization
// header with a Bearer token on every request. The tokenFunc is called once
// per request, which allows dynamic token refresh.
func WithBearerToken(tokenFunc func() string) ClientOption {
	return WithMiddleware(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			r.Header.Set("Authorization", "Bearer "+tokenFunc())
			return next.RoundTrip(r)
		})
	})
}

// WithStaticBearerToken returns a [ClientOption] that injects a fixed
// Authorization: Bearer header on every request.
func WithStaticBearerToken(token string) ClientOption {
	return WithBearerToken(func() string { return token })
}

// WithRequestID returns a [ClientOption] that propagates the X-Request-ID
// header from the request context. Use [WithRequestIDValue] to set the
// request ID in the context before calling the client.
func WithRequestID() ClientOption {
	return WithMiddleware(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			id := RequestIDFromContext(r.Context())
			if id != "" {
				r = r.Clone(r.Context())
				r.Header.Set("X-Request-ID", id)
			}
			return next.RoundTrip(r)
		})
	})
}

// WithHeader returns a [ClientOption] that injects a static header on every
// request.
func WithHeader(key, value string) ClientOption {
	return WithMiddleware(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			r.Header.Set(key, value)
			return next.RoundTrip(r)
		})
	})
}

// WithHeaders returns a [ClientOption] that injects multiple static headers
// on every request.
func WithHeaders(headers map[string]string) ClientOption {
	return WithMiddleware(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			for k, v := range headers {
				r.Header.Set(k, v)
			}
			return next.RoundTrip(r)
		})
	})
}

// WithLogging returns a [ClientOption] that logs the HTTP method, URL, response
// status code, and request duration using the provided [slog.Logger].
func WithLogging(logger *slog.Logger) ClientOption {
	return WithMiddleware(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.RoundTrip(r)
			duration := time.Since(start)
			if err != nil {
				logger.LogAttrs(r.Context(), slog.LevelError, "http request failed",
					slog.String("method", r.Method),
					slog.String("url", r.URL.String()),
					slog.Duration("duration", duration),
					slog.String("error", err.Error()),
				)
			} else {
				logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
					slog.String("method", r.Method),
					slog.String("url", r.URL.String()),
					slog.Int("status", resp.StatusCode),
					slog.Duration("duration", duration),
				)
			}
			return resp, err
		})
	})
}

// WithTimeout returns a [ClientOption] that enforces a per-request timeout
// using [context.WithTimeout]. If the request already has a shorter deadline,
// the existing deadline is preserved.
func WithTimeout(d time.Duration) ClientOption {
	return WithMiddleware(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			r = r.Clone(ctx)
			return next.RoundTrip(r)
		})
	})
}

// isIdempotent reports whether the HTTP method is considered idempotent and
// therefore safe to retry.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

// WithRetry returns a [Middleware] that retries requests on 5xx status codes or
// network errors. It uses exponential backoff (backoff * 2^attempt) between
// retries. Only idempotent methods (GET, HEAD, OPTIONS, PUT, DELETE) are
// retried; non-idempotent methods pass through without retry.
//
// maxAttempts is the total number of attempts including the initial request
// (e.g., 3 means one initial attempt plus up to two retries).
func WithRetry(maxAttempts int, backoff time.Duration) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if !isIdempotent(r.Method) {
				return next.RoundTrip(r)
			}

			var resp *http.Response
			var err error
			for attempt := 0; attempt < maxAttempts; attempt++ {
				if attempt > 0 {
					delay := backoff * (1 << (attempt - 1))
					select {
					case <-time.After(delay):
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
				}

				resp, err = next.RoundTrip(r)
				if err != nil {
					continue
				}
				if resp.StatusCode < 500 {
					return resp, nil
				}
				// Drain and close the body so the connection can be reused.
				if resp.Body != nil {
					resp.Body.Close()
				}
			}
			return resp, err
		})
	}
}

// WithMetrics returns a [Middleware] that calls fn after each request completes
// with the HTTP method, URL, response status code, and request duration. If the
// request fails with a network error, the status code passed to fn is 0.
func WithMetrics(fn func(method, url string, status int, duration time.Duration)) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.RoundTrip(r)
			elapsed := time.Since(start)
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			fn(r.Method, r.URL.String(), status, elapsed)
			return resp, err
		})
	}
}

// WithBaseURL returns a [ClientOption] that prepends baseURL to every request
// URL. Trailing slashes on baseURL and leading slashes on the request path are
// deduplicated so that exactly one slash separates them.
func WithBaseURL(baseURL string) ClientOption {
	return WithMiddleware(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			base := strings.TrimRight(baseURL, "/")
			path := strings.TrimLeft(r.URL.String(), "/")
			parsed, err := http.NewRequest(r.Method, base+"/"+path, r.Body)
			if err != nil {
				return nil, err
			}
			parsed.Header = r.Header
			return next.RoundTrip(parsed)
		})
	})
}

// WithOnRequest returns a [Middleware] that calls fn before each request is
// sent. The function receives a clone of the request for inspection or
// modification.
func WithOnRequest(fn func(*http.Request)) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			fn(r)
			return next.RoundTrip(r)
		})
	}
}

// WithOnResponse returns a [Middleware] that calls fn after each request
// completes successfully. The function receives the response for inspection.
// If the request fails with a network error, fn is not called.
func WithOnResponse(fn func(*http.Response)) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp, err := next.RoundTrip(r)
			if err != nil {
				return resp, err
			}
			fn(resp)
			return resp, nil
		})
	}
}
