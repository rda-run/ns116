package util

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the true client IP address from a request,
// considering X-Forwarded-For headers if present (e.g. from Ingress/Nginx).
func GetClientIP(r *http.Request) string {
	// 1. Check X-Forwarded-For (standard for proxies)
	// Format: client, proxy1, proxy2
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// Taking the first IP in the list as the client IP
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// 2. Check X-Real-IP (common alternative)
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// 3. Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return ip
	}

	return r.RemoteAddr
}
