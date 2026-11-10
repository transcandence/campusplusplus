package proxy

import (
	"fmt"
	"net/http/httputil"
	"net/url"
)

func parsTarget(rawURL string) (*url.URL, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return  nil, err
	}
	if (target.Scheme != "http" && target.Scheme != "https") || target.Host == ""{
		return nil, fmt.Errorf("URL must contain a valid HTTP/HTTPS scheme and host")
	}
	return target, nil
}

func NewProxy(identityURL, housingURL, transportURL string) (*httputil.ReverseProxy, error) {
	identityTarget, err := parsTarget(identityURL)
	if err != nil {
		return nil, fmt.Errorf("invalid IDENTITY_URL: %w", err)
	}
	housingTarget, err := parsTarget(housingURL)
	if err != nil {
		return  nil, fmt.Errorf("invalid HOUSING_URL: %w", err)
	}
	transportTarget, err := parsTarget(transportURL)
	if err != nil {
		return  nil, fmt.Errorf("invalid TRANSPORT_URL: %w", err)
	}
	routes := []route{	
		{
			prefix: "/api/v1/auth",
			target: identityTarget,
		},
		{
			prefix: "/api/v1/housing",
			target: housingTarget,
		},
		{
			prefix: "/api/v1/transport",
			target: transportTarget,
		},
	}
	proxy := &httputil.ReverseProxy{
		Director: newDirector(routes),
		ErrorHandler: newErrorHandeler(),
	}
	return proxy, nil
}