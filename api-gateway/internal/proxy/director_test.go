
package proxy

import (
    "fmt"
    "net/http" // ADD THIS
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
)

func printRequest(title string, req *http.Request) {
	fmt.Println()
	fmt.Println("==========", title, "==========")
	fmt.Println("Method:          ", req.Method)
	fmt.Println("URL:             ", req.URL.String())
	fmt.Println("URL Scheme:      ", req.URL.Scheme)
	fmt.Println("URL Host:        ", req.URL.Host)
	fmt.Println("URL Path:        ", req.URL.Path)
	fmt.Println("URL RawPath:     ", req.URL.RawPath)
	fmt.Println("Request Host:    ", req.Host)
	fmt.Println("RemoteAddr:      ", req.RemoteAddr)
	fmt.Println("Content-Type:    ", req.Header.Get("Content-Type"))
	fmt.Println("Authorization:   ", req.Header.Get("Authorization"))
	fmt.Println("X-Request-Id:    ", req.Header.Get("X-Request-Id"))
	fmt.Println("X-Forwarded-For: ", req.Header.Get("X-Forwarded-For"))
	fmt.Println("X-Forwarded-Proto:", req.Header.Get("X-Forwarded-Proto"))
	fmt.Println("X-Forwarded-Host:", req.Header.Get("X-Forwarded-Host"))
	fmt.Println("Headers:         ", req.Header)
	fmt.Println("====================================")
}

func TestDirectorDebug(t *testing.T) {
	// 1. Identity destination
	target, err := url.Parse("http://identity:8080")
	if err != nil {
		t.Fatal(err)
	}

	// 2. Configure Gateway routes
	routes := []route{
		{
			prefix: "/api/v1/auth",
			target: target,
		},
	}

	// 3. Simulate the request sent by Postman
	req := httptest.NewRequest(
		"POST",
		"http://localhost:8080/me",
		strings.NewReader(`{"email":"test@example.com","password":"123456"}`),
	)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer example-token")

	// Simulate the client's IP
	req.RemoteAddr = "192.168.1.10:54321"

	// 4. Print original request
	printRequest("BEFORE DIRECTOR (POSTMAN)", req)

	// 5. Execute your actual Director
	director := newDirector(routes)
	director(req)

	// 6. Print modified request
	printRequest("AFTER DIRECTOR (GATEWAY)", req)
}
