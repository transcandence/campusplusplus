
package proxy

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type route struct {
	prefix string
	target *url.URL
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
			if token = strings.TrimSpace(token); token != "" {
				header.Del(token)
			}
		}
	}
	for _, h := range hopbyHeaders {
		header.Del(h)
	}
}
func ensureRequestID(req *http.Request) {
	id := req.Header.Get("X-Request-ID")
	if _, err := uuid.Parse(id); err == nil {
		return
	}
	req.Header.Set("X-Request-ID", uuid.NewString())
}

func newDirector(routes []route) func(*http.Request) {
	return func(req *http.Request) {
		ensureRequestID(req)
		removeHopByHeaders(req.Header)
		for _, rt := range routes {
			if req.URL.Path != rt.prefix && !strings.HasPrefix(req.URL.Path, rt.prefix + "/") {
				continue
			}
			req.URL.Scheme = rt.target.Scheme
			req.URL.Host = rt.target.Host
			req.Host = rt.target.Host
			req.URL.Path = strings.TrimPrefix(req.URL.Path, rt.prefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.URL.RawPath = ""
			return
		}
	}
}