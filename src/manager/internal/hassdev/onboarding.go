package hassdev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
)

type onboardingStep struct {
	Step string `json:"step"`
	Done bool   `json:"done"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (c *httpClient) onboardingStatus(ctx context.Context) ([]onboardingStep, error) {
	data, err := c.get(ctx, "/api/onboarding", "")
	if err != nil {
		return nil, err
	}
	var steps []onboardingStep
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func onboardingComplete(steps []onboardingStep) bool {
	for _, s := range steps {
		if !s.Done {
			return false
		}
	}
	return len(steps) > 0
}

func stepDone(steps []onboardingStep, name string) bool {
	for _, s := range steps {
		if s.Step == name {
			return s.Done
		}
	}
	return false
}

func (c *httpClient) exchangeAuthCode(ctx context.Context, clientID, code string) (tokenResponse, error) {
	form := url.Values{
		"grant_type":       {"authorization_code"},
		"code":             {code},
		oauthFieldClientID: {clientID},
	}
	return c.exchangeTokenForm(ctx, form)
}

func (c *httpClient) exchangeRefreshToken(ctx context.Context, clientID, refresh string) (tokenResponse, error) {
	form := url.Values{
		"grant_type":       {"refresh_token"},
		"refresh_token":    {refresh},
		oauthFieldClientID: {clientID},
	}
	return c.exchangeTokenForm(ctx, form)
}

func (c *httpClient) exchangeTokenForm(ctx context.Context, form url.Values) (tokenResponse, error) {
	data, err := c.postForm(ctx, "/auth/token", form)
	if err != nil {
		return tokenResponse{}, err
	}
	var resp tokenResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return tokenResponse{}, err
	}
	return resp, nil
}

func RunOnboarding(ctx context.Context, cfg Config, creds *Credentials) error {
	http := newHTTPClient(cfg)
	steps, err := http.onboardingStatus(ctx)
	if err != nil {
		return err
	}

	tokens, err := completeUserOnboardingStep(ctx, cfg, creds, http, steps)
	if err != nil {
		return err
	}

	access := tokens.AccessToken
	refresh := tokens.RefreshToken
	if refresh == "" {
		refresh = creds.HARefreshToken
	}

	access, refresh, err = completeRemainingOnboardingSteps(ctx, cfg, http, access, refresh)
	if err != nil {
		return err
	}

	creds.HAURL = cfg.HAURL
	creds.HAUser = cfg.HAUser
	creds.HAPass = cfg.HAPass
	creds.HAClientID = cfg.HAClientID
	creds.HARefreshToken = refresh
	creds.HAAccessToken = access
	creds.OTBRURL = cfg.OTBRURL

	_, _ = fmt.Fprintf(os.Stdout, "==> Setting Home Assistant location (%s)\n", cfg.HACountry)
	if err := EnsureCoreConfig(ctx, cfg, access); err != nil {
		return fmt.Errorf("core config: %w", err)
	}
	return nil
}

func completeUserOnboardingStep(
	ctx context.Context,
	cfg Config,
	creds *Credentials,
	http *httpClient,
	steps []onboardingStep,
) (tokenResponse, error) {
	if stepDone(steps, "user") {
		return refreshExistingUserTokens(ctx, cfg, creds, http)
	}
	return createOnboardingUser(ctx, cfg, http)
}

func createOnboardingUser(ctx context.Context, cfg Config, http *httpClient) (tokenResponse, error) {
	_, _ = fmt.Fprintf(os.Stdout, "==> Creating admin user (%s)\n", cfg.HAUser)
	body := map[string]string{
		"name":             cfg.HAName,
		"username":         cfg.HAUser,
		"password":         cfg.HAPass,
		"language":         "en",
		oauthFieldClientID: cfg.HAClientID,
	}
	data, err := http.postJSON(ctx, "/api/onboarding/users", body, "")
	if err != nil {
		return tokenResponse{}, err
	}
	var userResp struct {
		AuthCode string `json:"auth_code"`
	}
	if err := json.Unmarshal(data, &userResp); err != nil {
		return tokenResponse{}, err
	}
	return http.exchangeAuthCode(ctx, cfg.HAClientID, userResp.AuthCode)
}

func refreshExistingUserTokens(ctx context.Context, cfg Config, creds *Credentials, http *httpClient) (tokenResponse, error) {
	_, _ = fmt.Fprintln(os.Stdout, "==> User step already complete")
	refresh := creds.HARefreshToken
	if refresh == "" {
		var err error
		refresh, err = ReadRefreshTokenFromStorage(cfg.HAConfigDir, cfg.HAClientID)
		if err != nil {
			return tokenResponse{}, fmt.Errorf("recover refresh token: %w", err)
		}
	}
	return http.exchangeRefreshToken(ctx, cfg.HAClientID, refresh)
}

func completeRemainingOnboardingSteps(
	ctx context.Context,
	cfg Config,
	http *httpClient,
	access, refresh string,
) (string, string, error) {
	var err error
	if access, err = finishOnboardingStep(ctx, http, access, "core_config", finishCoreConfigStep); err != nil {
		return "", "", err
	}
	if access, err = finishOnboardingStep(ctx, http, access, "analytics", finishAnalyticsStep); err != nil {
		return "", "", err
	}
	return finishIntegrationStep(ctx, cfg, http, access, refresh)
}

type onboardingStepFn func(context.Context, *httpClient, string) error

func finishOnboardingStep(
	ctx context.Context,
	http *httpClient,
	access, step string,
	finish onboardingStepFn,
) (string, error) {
	steps, err := http.onboardingStatus(ctx)
	if err != nil {
		return "", err
	}
	if stepDone(steps, step) {
		return access, nil
	}
	if err := finish(ctx, http, access); err != nil {
		return "", err
	}
	return access, nil
}

func finishCoreConfigStep(ctx context.Context, http *httpClient, access string) error {
	_, _ = fmt.Fprintln(os.Stdout, "==> Finishing core_config step")
	_, err := http.postJSON(ctx, "/api/onboarding/core_config", map[string]any{}, access)
	return err
}

func finishAnalyticsStep(ctx context.Context, http *httpClient, access string) error {
	_, _ = fmt.Fprintln(os.Stdout, "==> Finishing analytics step")
	body := map[string]bool{"analytics": false, "crash_reporting": false}
	_, err := http.postJSON(ctx, "/api/onboarding/analytics", body, access)
	return err
}

func finishIntegrationStep(
	ctx context.Context,
	cfg Config,
	http *httpClient,
	access, refresh string,
) (string, string, error) {
	steps, err := http.onboardingStatus(ctx)
	if err != nil {
		return "", "", err
	}
	if stepDone(steps, "integration") {
		return access, refresh, nil
	}
	_, _ = fmt.Fprintln(os.Stdout, "==> Finishing integration step")
	body := map[string]string{
		oauthFieldClientID: cfg.HAClientID,
		"redirect_uri":     cfg.HAClientID + "?auth_callback=1",
	}
	data, err := http.postJSON(ctx, "/api/onboarding/integration", body, access)
	if err != nil {
		return "", "", err
	}
	var integrationResp struct {
		AuthCode string `json:"auth_code"`
	}
	if err := json.Unmarshal(data, &integrationResp); err != nil {
		return "", "", err
	}
	tokens, err := http.exchangeAuthCode(ctx, cfg.HAClientID, integrationResp.AuthCode)
	if err != nil {
		return "", "", err
	}
	return tokens.AccessToken, tokens.RefreshToken, nil
}

func VerifyToken(ctx context.Context, cfg Config, token string) error {
	http := newHTTPClient(cfg)
	_, err := http.postJSON(ctx, "/api/template", map[string]string{"template": "ok"}, token)
	return err
}
