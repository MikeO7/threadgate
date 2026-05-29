package hass

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

// WS is a Home Assistant WebSocket session after auth.
type WS struct {
	conn *websocket.Conn
	id   int
}

// Dial opens and authenticates a Home Assistant WebSocket session.
func Dial(ctx context.Context, haURL, accessToken string) (*WS, error) {
	wsURL, err := buildWSURL(haURL)
	if err != nil {
		return nil, err
	}

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	c := &WS{conn: conn}

	var hello map[string]any
	if err := conn.ReadJSON(&hello); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if hello["type"] != "auth_required" {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected websocket hello: %v", hello)
	}
	if err := conn.WriteJSON(map[string]string{
		"type":         "auth",
		"access_token": accessToken,
	}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	var authOK map[string]any
	if err := conn.ReadJSON(&authOK); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if authOK["type"] != "auth_ok" {
		_ = conn.Close()
		return nil, fmt.Errorf("websocket auth failed: %v", authOK)
	}
	return c, nil
}

func buildWSURL(haURL string) (string, error) {
	u, err := url.Parse(strings.TrimSuffix(haURL, "/"))
	if err != nil {
		return "", err
	}
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	port := u.Port()
	if port == "" {
		if scheme == "wss" {
			port = "443"
		} else {
			port = "8123"
		}
	}
	return fmt.Sprintf("%s://%s:%s/api/websocket", scheme, u.Hostname(), port), nil
}

// Close ends the WebSocket session.
func (c *WS) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Call sends a typed WebSocket command and waits for the matching result object.
func (c *WS) Call(ctx context.Context, msgType string, fields map[string]any) (map[string]any, error) {
	raw, err := c.CallRaw(ctx, msgType, fields)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(raw, &result); err == nil {
		return result, nil
	}
	return map[string]any{}, nil
}

// CallRaw sends a typed WebSocket command and returns the raw JSON result.
func (c *WS) CallRaw(ctx context.Context, msgType string, fields map[string]any) (json.RawMessage, error) {
	c.id++
	payload := map[string]any{"id": c.id, "type": msgType}
	for k, v := range fields {
		payload[k] = v
	}
	if err := c.conn.WriteJSON(payload); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var raw map[string]json.RawMessage
		if err := c.conn.ReadJSON(&raw); err != nil {
			return nil, err
		}
		var id float64
		_ = json.Unmarshal(raw["id"], &id)
		if int(id) != c.id {
			continue
		}
		var success bool
		_ = json.Unmarshal(raw["success"], &success)
		if !success {
			b, _ := json.Marshal(raw)
			return nil, fmt.Errorf("%s failed: %s", msgType, string(b))
		}
		return raw["result"], nil
	}
}

// CreateLongLivedToken requests a long-lived access token over WebSocket.
func CreateLongLivedToken(ctx context.Context, haURL, accessToken, clientName string, lifespan int) (string, error) {
	c, err := Dial(ctx, haURL, accessToken)
	if err != nil {
		return "", err
	}
	defer func() { _ = c.Close() }()
	raw, err := c.CallRaw(ctx, "auth/long_lived_access_token", map[string]any{
		"client_name": clientName,
		"lifespan":    lifespan,
	})
	if err != nil {
		return "", err
	}
	var token string
	if err := json.Unmarshal(raw, &token); err == nil && token != "" {
		return token, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if t, ok := obj["access_token"].(string); ok && t != "" {
			return t, nil
		}
	}
	return "", fmt.Errorf("no access_token in websocket result: %s", string(raw))
}
