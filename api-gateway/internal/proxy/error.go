package proxy

import (
	"encoding/json"
	"net/http"
)

type ProblemDetails struct {
	Type 	string	`json:"type"`
	Title	string	`json:"title"`
	Status	int		`json:"status"`
	Code	string	`json:"code"`
	Detail	string	`json:"detail"`
	RequestID	string	`json:"requestId"`
}
func newErrorHandeler() func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		requestID := r.Header.Get("X-Request-Id")
		if r.URL.Host == "" {
			problem := ProblemDetails{
				Type:      "https://datatracker.ietf.org/doc/html/rfc9110#section-15.5.5",
				Title:     "Not Found",
				Status:    http.StatusNotFound,
				Code:      "not_found",
				Detail:    "No upstream route matches the requested path",
				RequestID: requestID,
			}
			w.Header().Set("Content-Type", "application/problem+json")
			w.Header().Set("X-Request-Id", requestID)
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(problem)
			return
		}
		problem := ProblemDetails{
			Type:      "about:blank",
			Title:     "Bad Gateway",
			Status:    http.StatusBadGateway,
			Code:      "bad_gateway",
			Detail:    "Upstream service is unreachable",
			RequestID: requestID,
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-Id", requestID)
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(problem)
	}
}