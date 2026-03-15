# go-httputil

Composable HTTP client middleware for Go. Build instrumented `http.Client` instances with zero hassle.

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
)

resp, err := client.Get("https://api.example.com/data")
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

### Request ID Propagation

Attach a request ID to the context, then let the middleware propagate it:

```go
ctx := httputil.WithRequestIDValue(context.Background(), "req-abc-123")
req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com", nil)
resp, err := client.Do(req)
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
| `WithRequestIDValue(ctx, id) context.Context` | Set request ID in context |
| `RequestIDFromContext(ctx) string` | Get request ID from context |
| `RoundTripperFunc` | Adapter type: function as http.RoundTripper |
| `Middleware` | Type: func(http.RoundTripper) http.RoundTripper |

## License

MIT
