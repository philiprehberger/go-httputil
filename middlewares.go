package httputil

import (
	"context"
	"log/slog"
	"net/http"
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
