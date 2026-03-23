package httputil

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	callCount := 0
	client := NewClient(
		WithBearerToken(func() string {
			callCount++
			return "dynamic-token"
		}),
	)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer dynamic-token" {
		t.Errorf("expected 'Bearer dynamic-token', got %q", gotAuth)
	}
	if callCount != 1 {
		t.Errorf("expected tokenFunc called once, got %d", callCount)
	}
}

func TestWithStaticBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithStaticBearerToken("my-static-token"))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer my-static-token" {
		t.Errorf("expected 'Bearer my-static-token', got %q", gotAuth)
	}
}

func TestWithHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithHeader("X-Custom", "custom-value"))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotHeader != "custom-value" {
		t.Errorf("expected 'custom-value', got %q", gotHeader)
	}
}

func TestWithHeaders(t *testing.T) {
	var gotA, gotB string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotA = r.Header.Get("X-Header-A")
		gotB = r.Header.Get("X-Header-B")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithHeaders(map[string]string{
		"X-Header-A": "value-a",
		"X-Header-B": "value-b",
	}))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotA != "value-a" {
		t.Errorf("expected 'value-a', got %q", gotA)
	}
	if gotB != "value-b" {
		t.Errorf("expected 'value-b', got %q", gotB)
	}
}

func TestWithRequestID(t *testing.T) {
	var gotID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithRequestID())

	ctx := WithRequestIDValue(context.Background(), "req-123")
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotID != "req-123" {
		t.Errorf("expected 'req-123', got %q", gotID)
	}
}

func TestWithRequestIDMissing(t *testing.T) {
	var gotID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithRequestID())

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotID != "" {
		t.Errorf("expected empty X-Request-ID, got %q", gotID)
	}
}

func TestRequestIDFromContext(t *testing.T) {
	ctx := context.Background()
	if id := RequestIDFromContext(ctx); id != "" {
		t.Errorf("expected empty string, got %q", id)
	}

	ctx = WithRequestIDValue(ctx, "abc")
	if id := RequestIDFromContext(ctx); id != "abc" {
		t.Errorf("expected 'abc', got %q", id)
	}
}

func TestWithLogging(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewClient(WithLogging(logger))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
}

func TestWithLoggingError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewClient(
		WithLogging(logger),
		WithBaseTransport(RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})),
	)

	_, err := client.Get("http://example.invalid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWithTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithTimeout(50 * time.Millisecond))

	_, err := client.Get(srv.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestWithTimeoutSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithTimeout(5 * time.Second))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
}

func TestWithRetryRecoversFrom500(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithMiddleware(WithRetry(3, 1*time.Millisecond)))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&count))
	}
}

func TestWithRetryNetworkError(t *testing.T) {
	attempts := 0
	client := NewClient(
		WithMiddleware(WithRetry(3, 1*time.Millisecond)),
		WithBaseTransport(RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			return nil, fmt.Errorf("connection refused")
		})),
	)

	_, err := client.Get("http://example.invalid")
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetrySkipsNonIdempotent(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(WithMiddleware(WithRetry(3, 1*time.Millisecond)))

	req, _ := http.NewRequest("POST", srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected 1 attempt for POST, got %d", atomic.LoadInt32(&count))
	}
}

func TestWithRetryRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(WithMiddleware(WithRetry(5, 10*time.Second)))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestWithRetryNoRetryOn200(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithMiddleware(WithRetry(3, 1*time.Millisecond)))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected 1 attempt, got %d", atomic.LoadInt32(&count))
	}
}

func TestWithMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	var gotMethod, gotURL string
	var gotStatus int
	var gotDuration time.Duration

	client := NewClient(WithMiddleware(WithMetrics(func(method, url string, status int, duration time.Duration) {
		gotMethod = method
		gotURL = url
		gotStatus = status
		gotDuration = duration
	})))

	resp, err := client.Get(srv.URL + "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotMethod != "GET" {
		t.Errorf("expected method GET, got %q", gotMethod)
	}
	if gotURL != srv.URL+"/test" {
		t.Errorf("expected URL %q, got %q", srv.URL+"/test", gotURL)
	}
	if gotStatus != http.StatusCreated {
		t.Errorf("expected status 201, got %d", gotStatus)
	}
	if gotDuration <= 0 {
		t.Errorf("expected positive duration, got %v", gotDuration)
	}
}

func TestWithMetricsOnError(t *testing.T) {
	var gotStatus int
	client := NewClient(
		WithMiddleware(WithMetrics(func(method, url string, status int, duration time.Duration) {
			gotStatus = status
		})),
		WithBaseTransport(RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network error")
		})),
	)

	_, err := client.Get("http://example.invalid")
	if err == nil {
		t.Fatal("expected error")
	}
	if gotStatus != 0 {
		t.Errorf("expected status 0 on error, got %d", gotStatus)
	}
}

func TestWithBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithBaseURL(srv.URL))

	resp, err := client.Get("/users/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := resp.Header.Get("X-Got-Path"); got != "/users/123" {
		t.Errorf("expected path /users/123, got %q", got)
	}
}

func TestWithBaseURLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Trailing slash on base URL, leading slash on path.
	client := NewClient(WithBaseURL(srv.URL + "/"))

	resp, err := client.Get("/items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := resp.Header.Get("X-Got-Path"); got != "/items" {
		t.Errorf("expected path /items, got %q", got)
	}
}

func TestWithBaseURLNoLeadingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithBaseURL(srv.URL))

	resp, err := client.Get("health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := resp.Header.Get("X-Got-Path"); got != "/health" {
		t.Errorf("expected path /health, got %q", got)
	}
}

func TestWithOnRequest(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Injected")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(WithMiddleware(WithOnRequest(func(r *http.Request) {
		r.Header.Set("X-Injected", "hook-value")
	})))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotHeader != "hook-value" {
		t.Errorf("expected 'hook-value', got %q", gotHeader)
	}
}

func TestWithOnResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Server", "test-server")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var gotHeader string
	client := NewClient(WithMiddleware(WithOnResponse(func(resp *http.Response) {
		gotHeader = resp.Header.Get("X-Server")
	})))

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotHeader != "test-server" {
		t.Errorf("expected 'test-server', got %q", gotHeader)
	}
}

func TestWithOnResponseNotCalledOnError(t *testing.T) {
	called := false
	client := NewClient(
		WithMiddleware(WithOnResponse(func(resp *http.Response) {
			called = true
		})),
		WithBaseTransport(RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection refused")
		})),
	)

	_, err := client.Get("http://example.invalid")
	if err == nil {
		t.Fatal("expected error")
	}
	if called {
		t.Error("expected OnResponse not to be called on network error")
	}
}
