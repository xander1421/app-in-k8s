package clients

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// HTTP3Client wraps an HTTP/3 client with fallback to HTTP/2
type HTTP3Client struct {
	http3Client *http.Client
	http2Client *http.Client
	baseURL     string
	timeout     time.Duration
}

// NewHTTP3Client creates a new HTTP/3 client with HTTP/2 fallback
func NewHTTP3Client(baseURL string, timeout time.Duration) *HTTP3Client {
	// HTTP/3 client configuration
	http3Transport := &http3.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // For dev - use proper certs in production
			NextProtos:         []string{"h3"},
		},
		QUICConfig: &quic.Config{
			MaxIdleTimeout:  30 * time.Second,
			EnableDatagrams: true,
		},
	}

	// HTTP/2 fallback client
	http2Transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // For dev - use proper certs in production
			NextProtos:         []string{"h2", "http/1.1"},
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &HTTP3Client{
		http3Client: &http.Client{
			Transport: http3Transport,
			Timeout:   timeout,
		},
		http2Client: &http.Client{
			Transport: http2Transport,
			Timeout:   timeout,
		},
		baseURL: baseURL,
		timeout: timeout,
	}
}

// Get performs an HTTP GET request with HTTP/3 and fallback
func (c *HTTP3Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, "GET", path, nil)
}

// Post performs an HTTP POST request with HTTP/3 and fallback
func (c *HTTP3Client) Post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}
	return c.doRequest(ctx, "POST", path, bodyReader)
}

// Put performs an HTTP PUT request with HTTP/3 and fallback
func (c *HTTP3Client) Put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}
	return c.doRequest(ctx, "PUT", path, bodyReader)
}

// Delete performs an HTTP DELETE request with HTTP/3 and fallback
func (c *HTTP3Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, "DELETE", path, nil)
}

// doRequest performs the actual HTTP request with HTTP/3 first, then HTTP/2 fallback
func (c *HTTP3Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path

	// Try HTTP/3 first
	resp, err := c.tryHTTP3Request(ctx, method, url, body)
	if err == nil {
		return resp, nil
	}

	// Fallback to HTTP/2
	return c.tryHTTP2Request(ctx, method, url, body)
}

// tryHTTP3Request attempts to make an HTTP/3 request
func (c *HTTP3Client) tryHTTP3Request(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP/3 request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "twitter-clone-http3/1.0")

	return c.http3Client.Do(req)
}

// tryHTTP2Request attempts to make an HTTP/2 request as fallback
func (c *HTTP3Client) tryHTTP2Request(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP/2 request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "twitter-clone-http2-fallback/1.0")

	return c.http2Client.Do(req)
}

// PostJSON is a convenience method for JSON POST requests
func (c *HTTP3Client) PostJSON(ctx context.Context, path string, body interface{}, result interface{}) error {
	resp, err := c.Post(ctx, path, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// GetJSON is a convenience method for JSON GET requests
func (c *HTTP3Client) GetJSON(ctx context.Context, path string, result interface{}) error {
	resp, err := c.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// Close closes the HTTP/3 client connections
func (c *HTTP3Client) Close() error {
	// Close HTTP/3 transport if possible
	if transport, ok := c.http3Client.Transport.(*http3.Transport); ok {
		transport.Close()
	}

	// Close HTTP/2 transport
	if transport, ok := c.http2Client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	return nil
}

// GetProtocol returns the protocol used in the last successful request
func (c *HTTP3Client) GetProtocol(resp *http.Response) string {
	if resp == nil {
		return "unknown"
	}
	return resp.Proto
}

// NewHTTP3ClientPool creates a pool of HTTP/3 clients for load balancing
type HTTP3ClientPool struct {
	clients []*HTTP3Client
	current int
}

// NewHTTP3ClientPool creates a pool of HTTP/3 clients
func NewHTTP3ClientPool(baseURLs []string, timeout time.Duration) *HTTP3ClientPool {
	pool := &HTTP3ClientPool{
		clients: make([]*HTTP3Client, len(baseURLs)),
	}

	for i, url := range baseURLs {
		pool.clients[i] = NewHTTP3Client(url, timeout)
	}

	return pool
}

// GetClient returns the next client in round-robin fashion
func (p *HTTP3ClientPool) GetClient() *HTTP3Client {
	if len(p.clients) == 0 {
		return nil
	}

	client := p.clients[p.current]
	p.current = (p.current + 1) % len(p.clients)
	return client
}

// Close closes all clients in the pool
func (p *HTTP3ClientPool) Close() error {
	var lastErr error
	for _, client := range p.clients {
		if err := client.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// HTTP3HealthChecker performs health checks over HTTP/3
type HTTP3HealthChecker struct {
	client   *HTTP3Client
	endpoint string
}

// NewHTTP3HealthChecker creates a new health checker
func NewHTTP3HealthChecker(baseURL, healthPath string) *HTTP3HealthChecker {
	return &HTTP3HealthChecker{
		client:   NewHTTP3Client(baseURL, 5*time.Second),
		endpoint: healthPath,
	}
}

// CheckHealth performs a health check
func (h *HTTP3HealthChecker) CheckHealth(ctx context.Context) error {
	resp, err := h.client.Get(ctx, h.endpoint)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}