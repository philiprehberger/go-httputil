# go-httputil

[![CI](https://github.com/philiprehberger/go-httputil/actions/workflows/ci.yml/badge.svg)](https://github.com/philiprehberger/go-httputil/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/philiprehberger/go-httputil.svg)](https://pkg.go.dev/github.com/philiprehberger/go-httputil) [![License](https://img.shields.io/github/license/philiprehberger/go-httputil)](LICENSE) [![Sponsor](https://img.shields.io/badge/sponsor-GitHub%20Sponsors-ec6cb9)](https://github.com/sponsors/philiprehberger)

Composable HTTP client middleware for Go. Build instrumented `http.Client` instances with zero hassle

## Installation

```bash
go get github.com/philiprehberger/go-httputil
```

## Usage

```go
import "github.com/philiprehberger/go-httputil"

client := httputil.NewClient(
    httputil.WithStaticBearerToken("my-api-key"),
    httputil.WithRequestID(),
    httputil.WithLogging(slog.Default()),
    httputil.WithTimeout(10 * time.Second),
    httputil.WithMiddleware(httputil.WithRetry(3, 500*time.Millisecond)),
    httputil.WithBaseURL("https://api.example.com"),
)

resp, err := client.Get("/data")
```

The returned `*http.Client` is standard — pass it anywhere an `http.Client` is accepted.

### Available Middleware

- **WithBearerToken(tokenFunc)** — inject a dynamic Authorization: Bearer header (called per-request)
- **WithStaticBearerToken(token)** — inject a fixed Bearer token
- **WithRequestID()** — propagate X-Request-ID from request context
- **WithHeader(key, value)** — inject a static header
- **WithHeaders(headers)** — inject multiple static headers
- **WithLogging(logger)** — log method, URL, status, and duration via slog
- **WithTimeout(duration)** — enforce a per-request timeout
- **WithRetry(maxAttempts, backoff)** — retry on 5xx/network errors with exponential backoff
- **WithMetrics(fn)** — call a function with method, URL, status, and duration after each request
- **WithBaseURL(baseURL)** — prepend a base URL to all requests
- **WithOnRequest(fn)** — pre-request hook for inspection/modification
- **WithOnResponse(fn)** — post-response hook for inspection

### Request ID Propagation

Attach a request ID to the context, then let the middleware propagate it:

```go
ctx := httputil.WithRequestIDValue(context.Background(), "req-abc-123")
req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com", nil)
resp, err := client.Do(req)
```

### Retry

Automatically retry failed requests with exponential backoff. Only idempotent methods (GET, HEAD, OPTIONS, PUT, DELETE) are retried:

```go
client := httputil.NewClient(
    httputil.WithMiddleware(httputil.WithRetry(3, 500*time.Millisecond)),
)

// Retries up to 3 total attempts on 5xx or network errors.
// Backoff: 500ms, 1s (exponential: backoff * 2^attempt).
resp, err := client.Get("https://api.example.com/data")
```

### Metrics

Collect request metrics by providing a callback:

```go
client := httputil.NewClient(
    httputil.WithMiddleware(httputil.WithMetrics(func(method, url string, status int, duration time.Duration) {
        log.Printf("%s %s -> %d (%s)", method, url, status, duration)
    })),
)
```

### Base URL

Prepend a base URL to all requests, with automatic slash deduplication:

```go
client := httputil.NewClient(
    httputil.WithBaseURL("https://api.example.com/v1"),
)

// Requests to "/users" become "https://api.example.com/v1/users"
resp, err := client.Get("/users")
```

### Hooks

Inspect or modify requests and responses with hook middleware:

```go
client := httputil.NewClient(
    httputil.WithMiddleware(httputil.WithOnRequest(func(r *http.Request) {
        r.Header.Set("X-Custom", "value")
    })),
    httputil.WithMiddleware(httputil.WithOnResponse(func(resp *http.Response) {
        log.Printf("response status: %d", resp.StatusCode)
    })),
)
```

### Custom Middleware

Build your own middleware using `WithMiddleware`:

```go
retryMiddleware := func(next http.RoundTripper) http.RoundTripper {
    return httputil.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
        resp, err := next.RoundTrip(r)
        if err != nil {
            return next.RoundTrip(r)
        }
        return resp, nil
    })
}

client := httputil.NewClient(
    httputil.WithMiddleware(retryMiddleware),
)
```

## API

| Function / Type | Description |
|-----------------|-------------|
| `NewClient(opts ...ClientOption) *http.Client` | Create an HTTP client with middleware chain |
| `WithMiddleware(m Middleware) ClientOption` | Append a custom middleware |
| `WithBaseTransport(rt http.RoundTripper) ClientOption` | Set the base transport (default: http.DefaultTransport) |
| `WithBearerToken(tokenFunc func() string) ClientOption` | Dynamic Bearer token per request |
| `WithStaticBearerToken(token string) ClientOption` | Fixed Bearer token |
| `WithRequestID() ClientOption` | Propagate X-Request-ID from context |
| `WithHeader(key, value string) ClientOption` | Inject a static header |
| `WithHeaders(headers map[string]string) ClientOption` | Inject multiple static headers |
| `WithLogging(logger *slog.Logger) ClientOption` | Log requests with slog |
| `WithTimeout(d time.Duration) ClientOption` | Per-request timeout |
| `WithRetry(maxAttempts int, backoff time.Duration) Middleware` | Retry on 5xx/network errors with exponential backoff |
| `WithMetrics(fn func(method, url string, status int, duration time.Duration)) Middleware` | Collect request metrics via callback |
| `WithBaseURL(baseURL string) ClientOption` | Prepend base URL to all requests |
| `WithOnRequest(fn func(*http.Request)) Middleware` | Pre-request hook |
| `WithOnResponse(fn func(*http.Response)) Middleware` | Post-response hook |
| `WithRequestIDValue(ctx, id) context.Context` | Set request ID in context |
| `RequestIDFromContext(ctx) string` | Get request ID from context |
| `RoundTripperFunc` | Adapter type: function as http.RoundTripper |
| `Middleware` | Type: func(http.RoundTripper) http.RoundTripper |

## Development

```bash
go test ./...
go vet ./...
```

## License

MIT
