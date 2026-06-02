package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMetricsTestServer(t *testing.T) *HTTPServer {
	t.Helper()
	return NewHTTPServer(Config{
		Addr:        ":0",
		FrontendDir: t.TempDir(),
	})
}

func TestMetricsEndpointReturnsStructuredJSON(t *testing.T) {
	srv := newMetricsTestServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/internal/metrics")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body MetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if body.RuntimeInternals.Scheduler.Goroutines <= 0 {
		t.Error("expected goroutines > 0")
	}
	if body.RuntimeInternals.Heap.SysBytes == 0 {
		t.Error("expected heap sys bytes > 0")
	}
	if body.ApplicationIO.UptimeSeconds < 0 {
		t.Errorf("negative uptime: %f", body.ApplicationIO.UptimeSeconds)
	}
	// Note: httptest.NewServer wraps the handler in its own http.Server, so our
	// ConnState callback isn't exercised here — connection counting is verified
	// directly in TestConnStateCounter below.
	if body.ApplicationIO.ActiveHTTPConnections < 0 {
		t.Errorf("active connections negative: %d", body.ApplicationIO.ActiveHTTPConnections)
	}
}

func TestConnStateCounter(t *testing.T) {
	srv := newMetricsTestServer(t)

	if got := srv.activeConns.Load(); got != 0 {
		t.Fatalf("initial active conns = %d, want 0", got)
	}

	// New connection opens, becomes active, then idle (keep-alive): count = 1.
	srv.trackConnState(nil, http.StateNew)
	srv.trackConnState(nil, http.StateActive)
	srv.trackConnState(nil, http.StateIdle)
	if got := srv.activeConns.Load(); got != 1 {
		t.Errorf("after open: active conns = %d, want 1", got)
	}

	// A second connection that gets hijacked (e.g. a WebSocket upgrade).
	srv.trackConnState(nil, http.StateNew)
	srv.trackConnState(nil, http.StateHijacked)
	if got := srv.activeConns.Load(); got != 1 {
		t.Errorf("after hijack: active conns = %d, want 1", got)
	}

	// First connection closes: back to 0.
	srv.trackConnState(nil, http.StateClosed)
	if got := srv.activeConns.Load(); got != 0 {
		t.Errorf("after close: active conns = %d, want 0", got)
	}
}

func TestMetricsEndpointRejectsNonGET(t *testing.T) {
	srv := newMetricsTestServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/internal/metrics", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestMetricsEndpointTokenGuard(t *testing.T) {
	srv := newMetricsTestServer(t)
	srv.metricsToken = "s3cret" // simulate METRICS_TOKEN being set
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	// No token -> 401.
	resp, err := http.Get(ts.URL + "/api/internal/metrics")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without token: status = %d, want 401", resp.StatusCode)
	}

	// Correct bearer token -> 200.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/internal/metrics", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("with token: status = %d, want 200", resp2.StatusCode)
	}
}
