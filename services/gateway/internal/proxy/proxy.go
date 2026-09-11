// Package proxy forwards a request to one upstream service. The gateway
// mounts one of these per service prefix, behind http.StripPrefix, so the
// upstream sees the path it has always served.
package proxy

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// New returns a handler that reverse-proxies to upstream. responseTimeout
// bounds how long the upstream may take to start responding; past it the
// caller gets 504, and any other transport failure gets 502. Upstream
// error text is never forwarded to the caller.
func New(upstream *url.URL, responseTimeout time.Duration) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(upstream)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = responseTimeout
	rp.Transport = transport

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		status, message := http.StatusBadGateway, "upstream unavailable"
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			status, message = http.StatusGatewayTimeout, "upstream timeout"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	}

	return rp
}
