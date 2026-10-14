package middleware

import (
	"net/http"
	"strings"
)

// ContentSecurityPolicy restricts the app shell to its own origin. Article
// images and media may come from anywhere (that is the nature of RSS), but
// scripts, styles, fonts and network requests are same-origin only, so
// sanitizer bypasses in feed content cannot load or exfiltrate anything.
const ContentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self'; " +
	"img-src * data: blob:; " +
	"media-src * blob:; " +
	"frame-src https:; " +
	"connect-src 'self'; " +
	"font-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"manifest-src 'self'; " +
	"worker-src 'self'"

// SecurityHeaders adds defensive response headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", ContentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		// Do not leak the address of a private instance (or the article being
		// read) to the sites whose images and links appear in feeds.
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// CrossOrigin rejects state-changing browser requests that originate from
// another site (CSRF), using the Sec-Fetch-Site / Origin headers via Go's
// http.CrossOriginProtection. Requests without those headers (curl, native
// Fever clients) are allowed, as they cannot carry a victim's cookies.
// The Fever API authenticates with an api_key form field rather than
// cookies, so it is exempt so that web-based Fever clients keep working.
//
// It must wrap the handler that sees paths with the base path already
// stripped (see Base).
func CrossOrigin(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	cop.AddInsecureBypassPattern("/fever/")
	return cop.Handler(next)
}

// IsSecureRequest reports whether the client reached us over HTTPS, either
// directly or through a TLS-terminating reverse proxy such as Tailscale
// Serve, Caddy or nginx. The header only influences the cookie Secure flag,
// so a spoofed value cannot weaken anything beyond what plain HTTP allows.
func IsSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Forwarded")), "proto=https")
}
