package commands

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Trusted browser origins allowed for cross-origin API calls (local Vite + production).
var trustedCORSOrigins = []string{
	"https://genesisrk.mftools.xyz",
	"https://genesis-app.bravecoast-8b272aef.westeurope.azurecontainerapps.io",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
	"http://localhost:8080",
	"http://127.0.0.1:8080",
}

func init() {
	if extra := strings.TrimSpace(os.Getenv("GENESIS_CORS_ORIGINS")); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				trustedCORSOrigins = append(trustedCORSOrigins, o)
			}
		}
	}
}

func isTrustedCORSOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	for _, o := range trustedCORSOrigins {
		if origin == o {
			return true
		}
	}
	return false
}

// securityHeadersMiddleware adds browser security headers and scoped CORS.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w, r)
		applyCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	// HSTS: only meaningful over HTTPS (Azure terminates TLS at the edge).
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("X-DNS-Prefetch-Control", "off")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy())

	// Avoid caching HTML / API with sensitive selections; assets keep their own Cache-Control.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if w.Header().Get("Cache-Control") == "" {
			w.Header().Set("Cache-Control", "no-store")
		}
	}
}

func contentSecurityPolicy() string {
	// Vue needs 'unsafe-inline' styles; Swagger UI loads from unpkg; icons/fonts from CDNs.
	directives := []string{
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"script-src 'self' https://unpkg.com",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://unpkg.com",
		"font-src 'self' https://fonts.gstatic.com data:",
		"img-src 'self' data: blob: https:",
		"connect-src 'self' https://endoflife.date https://cdn.jsdelivr.net https://unpkg.com https://fonts.googleapis.com https://fonts.gstatic.com",
		"worker-src 'self' blob:",
		"upgrade-insecure-requests",
	}
	return strings.Join(directives, "; ")
}

func applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if !isTrustedCORSOrigin(origin) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Vary", "Origin")
}

const securityTxtBody = `Contact: mailto:ala.eltai@suse.com
Contact: https://github.com/aeltai/GenesisRK/issues
Expires: 2027-12-31T23:59:59.000Z
Preferred-Languages: en
Canonical: https://genesisrk.mftools.xyz/.well-known/security.txt
Policy: https://github.com/aeltai/GenesisRK
Hiring:
`

const robotsTxtBody = `User-agent: *
Allow: /
Disallow: /api/generate
Disallow: /api/export
Disallow: /api/scan
Disallow: /api/logs
Sitemap: https://genesisrk.mftools.xyz/
`

func handleSecurityTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = fmt.Fprint(w, securityTxtBody)
}

func handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = fmt.Fprint(w, robotsTxtBody)
}

// requestURLHost helps build absolute URLs when needed.
func requestURLHost(r *http.Request) string {
	if r == nil {
		return "genesisrk.mftools.xyz"
	}
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return h
	}
	if r.Host != "" {
		return r.Host
	}
	u, err := url.Parse("https://genesisrk.mftools.xyz")
	if err != nil {
		return "genesisrk.mftools.xyz"
	}
	return u.Host
}
