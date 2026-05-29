// Package pairing coordinates Home Assistant link approval and HTTP client helpers.
package pairing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient calls ThreadGate pairing endpoints for integration tooling.
type HTTPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Initiate starts a pending pairing request on ThreadGate.
func (c *HTTPClient) Initiate(ctx context.Context, appName, appURL, hassToken string) (string, error) {
	client := c.http()
	body, _ := json.Marshal(map[string]string{
		"app_name":   appName,
		"app_url":    appURL,
		"hass_token": hassToken,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/api/pair/initiate"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("pair initiate: %s", string(data))
	}
	var init struct {
		PairingID string `json:"pairing_id"`
	}
	if err := json.Unmarshal(data, &init); err != nil {
		return "", err
	}
	if init.PairingID == "" {
		return "", fmt.Errorf("pair initiate: missing pairing_id")
	}
	return init.PairingID, nil
}

// WaitForApproval blocks until the user approves in the ThreadGate dashboard or timeout.
func (c *HTTPClient) WaitForApproval(ctx context.Context, pairingID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := c.http()
	statusURL := c.url("/api/pair/status?id=" + pairingID)

	for {
		if time.Now().After(deadline) {
			return approvalTimeoutError(c.BaseURL)
		}
		status, err := c.fetchPairStatus(ctx, client, statusURL)
		if err != nil {
			return err
		}
		if done, waitErr := handlePairStatus(status); done {
			return waitErr
		}
		if err := waitForNextPoll(ctx); err != nil {
			return err
		}
	}
}

func approvalTimeoutError(baseURL string) error {
	return fmt.Errorf("timed out waiting for approval (open %s and click Approve Connection)", strings.TrimSuffix(baseURL, "/"))
}

func (c *HTTPClient) fetchPairStatus(ctx context.Context, client *http.Client, statusURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("pair status: %s", string(body))
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return "", err
	}
	return status.Status, nil
}

func handlePairStatus(status string) (done bool, err error) {
	switch status {
	case StatusApproved:
		return true, nil
	case StatusDenied:
		return true, fmt.Errorf("pairing denied in ThreadGate dashboard")
	case "expired":
		return true, fmt.Errorf("pairing request expired")
	case StatusPending:
		return false, nil
	default:
		return true, fmt.Errorf("unexpected pairing status: %q", status)
	}
}

func waitForNextPoll(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return nil
	}
}

// Approve auto-approves a pending pairing request (integration tooling).
func (c *HTTPClient) Approve(ctx context.Context, pairingID string) error {
	client := c.http()
	body, _ := json.Marshal(map[string]string{"pairing_id": pairingID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/api/pair/approve"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pair approve: %s", string(data))
	}
	return nil
}

func (c *HTTPClient) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *HTTPClient) url(path string) string {
	return strings.TrimSuffix(c.BaseURL, "/") + path
}
