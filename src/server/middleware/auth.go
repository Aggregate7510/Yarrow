package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

func IsAuthenticated(req *http.Request, username, password string) bool {
	cookie, _ := req.Cookie("auth")
	if cookie == nil {
		return false
	}
	parts := strings.Split(cookie.Value, ":")
	if len(parts) != 2 || !StringsEqual(parts[0], username) {
		return false
	}
	return StringsEqual(parts[1], secret(username, password))
}

// Authenticate sets the session cookie. The Secure flag follows the scheme
// the browser actually used (directly or via a TLS-terminating proxy), so
// login works both on plain-HTTP LAN addresses and behind HTTPS proxies
// such as Tailscale Serve, while never being sent in the clear once HTTPS
// is in use.
func Authenticate(rw http.ResponseWriter, r *http.Request, username, password, basepath string) {
	http.SetCookie(rw, &http.Cookie{
		Name:     "auth",
		Value:    username + ":" + secret(username, password),
		MaxAge:   604800, // 1 week
		Path:     cookiePath(basepath),
		Secure:   IsSecureRequest(r),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func Logout(rw http.ResponseWriter, r *http.Request, basepath string) {
	http.SetCookie(rw, &http.Cookie{
		Name:     "auth",
		Value:    "",
		MaxAge:   -1,
		Path:     cookiePath(basepath),
		Secure:   IsSecureRequest(r),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func cookiePath(basepath string) string {
	if basepath == "" {
		return "/"
	}
	return "/" + strings.Trim(basepath, "/")
}

func StringsEqual(p1, p2 string) bool {
	return subtle.ConstantTimeCompare([]byte(p1), []byte(p2)) == 1
}

func secret(msg, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	src := mac.Sum(nil)
	return hex.EncodeToString(src)
}

func LocalAuth(next http.Handler, username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if IsAuthenticated(r, username, password) {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}
}
