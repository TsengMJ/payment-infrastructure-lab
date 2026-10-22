package main

import (
	"net"
	"net/http"
)

func newMetricsHandler(metrics http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if metrics == nil || !internalCaller(r.RemoteAddr) {
			http.Error(w, "forbidden\n", http.StatusForbidden)
			return
		}
		metrics.ServeHTTP(w, r)
	})
}

func internalCaller(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
