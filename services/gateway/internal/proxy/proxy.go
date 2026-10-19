// Package proxy forwards a request to one upstream service. The gateway
// mounts one of these per service prefix, behind http.StripPrefix, so the
// upstream sees the path it has always served.
package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/phuoctmse/settleguard/gateway/internal/httperror"
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

	// The Server header names an upstream's software and version (SG-PT-09).
	rp.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Server")
		return nil
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		status, message := http.StatusBadGateway, "upstream unavailable"
		var netErr net.Error
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			// The caller's body overran the gateway's cap mid-stream (SG-PT-08).
			status, message = http.StatusRequestEntityTooLarge, "request body too large"
		case errors.As(err, &netErr) && netErr.Timeout():
			status, message = http.StatusGatewayTimeout, "upstream timeout"
		}
		httperror.Write(w, status, message)
	}

	return rp
}
