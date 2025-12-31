package docker

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProxyRequest proxies an HTTP request to a container port
// deploymentPath is the path prefix to strip from the request (e.g., "/test/html-server")
func ProxyRequest(req *http.Request, hostPort int, deploymentPath string) (*http.Response, error) {
	// Create a new request to the container
	containerURL := req.URL
	containerURL.Scheme = "http"
	containerURL.Host = fmt.Sprintf("127.0.0.1:%d", hostPort)

	// Strip the deployment path from the request path
	// e.g., /test/html-server/api/users -> /api/users
	// e.g., /test/html-server -> /
	path := req.URL.Path
	if strings.HasPrefix(path, deploymentPath) {
		path = path[len(deploymentPath):]
		if path == "" {
			path = "/"
		}
	}
	containerURL.Path = path
	containerURL.RawQuery = req.URL.RawQuery

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create new request
	proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, containerURL.String(), req.Body)
	if err != nil {
		return nil, fmt.Errorf("create proxy request: %w", err)
	}

	// Copy headers (exclude hop-by-hop headers)
	for key, values := range req.Header {
		if !isHopByHopHeader(key) {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}
	}

	// Set host header
	proxyReq.Host = containerURL.Host

	// Execute request
	resp, err := client.Do(proxyReq)
	if err != nil {
		return nil, fmt.Errorf("proxy request: %w", err)
	}

	return resp, nil
}

// ProxyResponse writes a proxied response to the response writer
func ProxyResponse(w http.ResponseWriter, resp *http.Response) error {
	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy headers (exclude hop-by-hop headers)
	for key, values := range resp.Header {
		if !isHopByHopHeader(key) {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Copy body
	if resp.Body != nil {
		defer resp.Body.Close()
		_, err := io.Copy(w, resp.Body)
		return err
	}

	return nil
}

// isHopByHopHeader checks if a header is a hop-by-hop header that shouldn't be proxied
func isHopByHopHeader(header string) bool {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	for _, h := range hopByHopHeaders {
		if header == h {
			return true
		}
	}
	return false
}
