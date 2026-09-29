package commands

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := securityHeadersMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "https://genesisrk.mftools.xyz/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	want := map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}
	for k, v := range want {
		if got := rr.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	csp := rr.Header().Get("Content-Security-Policy")
	for _, part := range []string{"default-src 'self'", "frame-ancestors 'none'", "upgrade-insecure-requests"} {
		if !strings.Contains(csp, part) {
			t.Errorf("CSP missing %q: %s", part, csp)
		}
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("unexpected CORS origin without Origin header")
	}
}

func TestCORSAllowlist(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := securityHeadersMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "https://genesisrk.mftools.xyz/api/logs", nil)
	req.Header.Set("Origin", "https://genesisrk.mftools.xyz")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://genesisrk.mftools.xyz" {
		t.Fatalf("trusted origin CORS = %q", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "https://genesisrk.mftools.xyz/api/logs", nil)
	req2.Header.Set("Origin", "https://evil.example")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if got := rr2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("untrusted origin should be blocked, got %q", got)
	}
}

func TestSecurityTxt(t *testing.T) {
	rr := httptest.NewRecorder()
	handleSecurityTxt(rr, httptest.NewRequest(http.MethodGet, "/.well-known/security.txt", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Contact:") {
		t.Fatalf("missing Contact: %s", rr.Body.String())
	}
}
