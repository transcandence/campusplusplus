package proxy

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type route struct {
	prefix      string
	target      *url.URL
}

var hopbyHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func removeHopByHeaders(header http.Header) {
	for _, h := range header.Values("Connection") {
		for _, token := range strings.Split(h, ",") {
			token = strings.TrimSpace(token)
			if strings.EqualFold(token, "X-Request-Id") || strings.EqualFold(token, "Authorization") {
				continue
			}
			if token != "" {
				header.Del(token)
			}
		}
	}
	for _, h := range hopbyHeaders {
		header.Del(h)
	}
}

func ensureRequestID(req *http.Request) {
	id := req.Header.Get("X-Request-Id")
	if _, err := uuid.Parse(id); err == nil {
		return
	}
	req.Header.Set("X-Request-Id", uuid.NewString())
}

func newDirector(routes []route) func(*http.Request) {
	return func(req *http.Request) {
		removeHopByHeaders(req.Header)
		ensureRequestID(req)

		for _, rt := range routes {
			if req.URL.Path != rt.prefix && !strings.HasPrefix(req.URL.Path, rt.prefix+"/") {
				continue
			}

			req.URL.Scheme = rt.target.Scheme
			req.URL.Host = rt.target.Host
			req.Host = rt.target.Host

			trimmed := strings.TrimPrefix(req.URL.Path, rt.prefix)
			if !strings.HasPrefix(trimmed, "/") {
				trimmed = "/" + trimmed
			}

			basePath := strings.TrimSuffix(rt.target.Path, "/")
			req.URL.Path = basePath + trimmed
			req.URL.RawPath = ""

			break
		}

		if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			if prior, ok := req.Header["X-Forwarded-For"]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
			req.Header.Set("X-Forwarded-For", clientIP)
		}
		if req.Header.Get("X-Forwarded-Proto") == "" {
			if req.TLS != nil {
				req.Header.Set("X-Forwarded-Proto", "https")
			} else {
				req.Header.Set("X-Forwarded-Proto", "http")
			}
		}
		if req.Header.Get("X-Forwarded-Host") == "" {
			req.Header.Set("X-Forwarded-Host", req.Host)
		}
	}
}