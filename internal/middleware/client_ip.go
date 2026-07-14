package middleware

import (
	"net/http"
	"strings"
)

func ClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.Header.Get("X-Real-IP")
		}
		if ip == "" {
			ip = r.RemoteAddr
		}
		if strings.Contains(ip, ",") {
			ip = strings.TrimSpace(strings.Split(ip, ",")[0])
		}
		r.Header.Set("X-Real-IP", ip)
		next.ServeHTTP(w, r)
	})
}
