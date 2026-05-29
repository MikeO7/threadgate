package hassdev

import (
	"context"
	"encoding/json"
	"fmt"
)

func configureOTBRHTTP(ctx context.Context, cfg Config, token string) (string, error) {
	http := newHTTPClient(cfg)
	startBody := map[string]any{
		"handler":               haDomainOTBR,
		"show_advanced_options": false,
	}
	data, err := http.postJSON(ctx, "/api/config/config_entries/flow", startBody, token)
	if err != nil {
		return "", err
	}
	var start struct {
		FlowID string `json:"flow_id"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(data, &start); err != nil {
		return "", err
	}
	if start.FlowID == "" {
		return string(data), nil
	}
	configureBody := map[string]any{
		"url": cfg.OTBRURL,
	}
	data, err = http.postJSON(ctx, "/api/config/config_entries/flow/"+start.FlowID, configureBody, token)
	if err != nil {
		return "", err
	}
	var result struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return string(data), nil
	}
	return result.Type, nil
}

func configureOTBR(ctx context.Context, cfg Config, token, otbrURL string) (string, error) {
	_ = otbrURL
	cfgCopy := cfg
	if otbrURL != "" {
		cfgCopy.OTBRURL = otbrURL
	}
	result, err := configureOTBRHTTP(ctx, cfgCopy, token)
	if err == nil {
		return result, nil
	}
	// Fall back to websocket for older HA builds.
	return configureOTBRWS(ctx, cfgCopy.HAURL, token, cfgCopy.OTBRURL)
}

func configureOTBRWS(ctx context.Context, haURL, token, otbrURL string) (string, error) {
	c, err := dialWS(ctx, haURL, token)
	if err != nil {
		return "", err
	}
	defer func() { _ = c.Close() }()
	flow, err := c.call(ctx, "config_entries/flow/create", map[string]any{
		"handler":               haDomainOTBR,
		"show_advanced_options": false,
	})
	if err != nil {
		return "", fmt.Errorf("websocket flow create: %w", err)
	}
	flowID, _ := flow["flow_id"].(string)
	result, err := c.call(ctx, "config_entries/flow/configure", map[string]any{
		"flow_id":    flowID,
		"user_input": map[string]string{"url": otbrURL},
	})
	if err != nil {
		return "", err
	}
	if t, ok := result["type"].(string); ok {
		return t, nil
	}
	return fmt.Sprintf("%v", result), nil
}
