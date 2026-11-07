package proxy

import (
	"net/http"
	"net/url"
	"strings"
)

type route struct {
    prefix string
    target *url.URL
}

func newDirector(routes []route) func (*http.Request) {
	return  func (req *http.Request){
		for _, rt := range routes {
			if !strings.HasPrefix(req.URL.Path, rt.prefix){
				continue;
			}
			req.URL.Scheme = rt.target.Scheme
			req.URL.Host =  rt.target.Host
			req.Host = rt.target.Host
			
			req.URL.Path = strings.TrimPrefix(req.URL.Path, rt.prefix)
			req.URL.RawPath = ""
			return 
		}
	}	
}