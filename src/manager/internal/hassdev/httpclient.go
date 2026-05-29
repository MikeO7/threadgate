package hassdev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type httpClient struct {
	base   string
	client *http.Client
}

func newHTTPClient(cfg Config) *httpClient {
	return &httpClient{
		base: strings.TrimSuffix(cfg.HAURL, "/"),
		client: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}
}

func (c *httpClient) get(ctx context.Context, path string, token string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil, token)
}

func (c *httpClient) postJSON(ctx context.Context, path string, body any, token string) ([]byte, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}
	return c.do(ctx, http.MethodPost, path, &buf, token)
}

func (c *httpClient) delete(ctx context.Context, path string, token string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, path, nil, token)
}

func (c *httpClient) postForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s", req.Method, path, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (c *httpClient) do(ctx context.Context, method, path string, body io.Reader, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s", method, path, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// WaitURL blocks until url returns HTTP 200 or 302.
func WaitURL(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for %s", url)
}
