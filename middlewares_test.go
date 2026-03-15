package httputil

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
