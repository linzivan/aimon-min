package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"ai-monitor/internal/types"
)

// DeepSeekProvider implements the Provider interface for DeepSeek API.
// V1: monitors balance via /user/balance.
// Token usage tracking TBD - requires intercepting chat completion responses.
type DeepSeekProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewDeepSeek(apiKey, baseURL string) *DeepSeekProvider {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	return &DeepSeekProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:      2,
				IdleConnTimeout:  30 * time.Second,
				ForceAttemptHTTP2: true,
			},
		},
	}
}

func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

// Collect fetches balance from DeepSeek API.
// Token usage from chat completions is not available via a query endpoint;
// it must be collected from chat completion responses at call time.
func (p *DeepSeekProvider) Collect(ctx context.Context) (*types.Metrics, error) {
	m := &types.Metrics{
		ProviderName:  "deepseek",
		Currency:      "CNY",
		AccountStatus: "active",
		CollectedAt:   time.Now(),
	}

	// Get balance from /user/balance
	balance, available, err := p.getBalance(ctx)
	if err != nil {
		return m, fmt.Errorf("get balance: %w", err)
	}
	m.Balance = balance
	if !available {
		m.AccountStatus = "unavailable"
	}

	// Token usage query endpoint does not exist in DeepSeek public API.
	// Set to 0 for V1; future versions can track from chat completion responses.
	m.TodayTokens = 0
	m.MonthTokens = 0

	return m, nil
}

// getBalance calls DeepSeek /user/balance endpoint.
// Official doc: https://api-docs.deepseek.com/zh-cn/api/get-user-balance
//
// Response:
//
//	{
//	  "is_available": true,
//	  "balance_infos": [
//	    {
//	      "currency": "CNY",
//	      "total_balance": "110.00",
//	      "granted_balance": "10.00",
//	      "topped_up_balance": "100.00"
//	    }
//	  ]
//	}
func (p *DeepSeekProvider) getBalance(ctx context.Context) (float64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		p.baseURL+"/user/balance", nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, false, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, false, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != 200 {
		return 0, false, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		IsAvailable  bool `json:"is_available"`
		BalanceInfos []struct {
			Currency       string `json:"currency"`
			TotalBalance   string `json:"total_balance"`
			GrantedBalance string `json:"granted_balance"`
			ToppedUpBalance string `json:"topped_up_balance"`
		} `json:"balance_infos"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, false, fmt.Errorf("parse: %w", err)
	}

	// Find CNY or USD balance
	for _, info := range result.BalanceInfos {
		if info.Currency == "CNY" || info.Currency == "USD" {
			var bal float64
			fmt.Sscanf(info.TotalBalance, "%f", &bal)
			return bal, result.IsAvailable, nil
		}
	}

	return 0, result.IsAvailable, fmt.Errorf("no balance info found")
}

func (p *DeepSeekProvider) Close() {
	p.client.CloseIdleConnections()
}
