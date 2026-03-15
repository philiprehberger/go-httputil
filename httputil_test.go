package httputil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Transport == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestNewClientWithBaseTransport(t *testing.T) {
	base := &http.Transport{}
	client := NewClient(WithBaseTransport(base))
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestMiddlewareOrder(t *testing.T) {
	var order []int

	mw := func(id int) Middleware {
		return func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				order = append(order, id)
				return next.RoundTrip(r)
			})
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(
		WithMiddleware(mw(1)),
		WithMiddleware(mw(2)),
		WithMiddleware(mw(3)),
	)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if len(order) != 3 {
		t.Fatalf("expected 3 middleware calls, got %d", len(order))
	}
	for i, id := range order {
		expected := i + 1
		if id != expected {
			t.Errorf("middleware %d executed at position %d, expected %d", id, i, expected)
		}
	}
}

func TestRoundTripperFunc(t *testing.T) {
	called := false
	f := RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return nil, fmt.Errorf("test")
	})

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _ = f.RoundTrip(req)

	if !called {
		t.Fatal("expected RoundTripperFunc to be called")
	}
}
