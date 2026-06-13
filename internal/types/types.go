package types

import (
	"context"
	"time"
)

// Metrics represents a snapshot of provider metrics.
type Metrics struct {
	ProviderName  string    `json:"provider_name"`
	Balance       float64   `json:"balance"`
	Currency      string    `json:"currency"`
	AccountStatus string    `json:"account_status"`
	TodayTokens   int64     `json:"today_tokens"`
	MonthTokens   int64     `json:"month_tokens"`
	CollectedAt   time.Time `json:"collected_at"`
	Error         string    `json:"error,omitempty"`
}

// Provider defines the interface all AI providers must implement.
// V1 only implements DeepSeekProvider. Other providers will be added later.
type Provider interface {
	// Name returns the provider identifier (e.g., "deepseek").
	Name() string

	// Collect fetches metrics from the provider API.
	// Returns nil metrics and no error on success.
	// Returns non-nil error on API failure.
	Collect(ctx context.Context) (*Metrics, error)
}

// AlertType categorizes notifications.
type AlertType int

const (
	AlertBalanceLow  AlertType = iota // Balance below threshold
	AlertAPIError                     // API call failed
	AlertTokenSurge                   // Unusual token consumption spike
)

func (a AlertType) String() string {
	switch a {
	case AlertBalanceLow:
		return "balance_low"
	case AlertAPIError:
		return "api_error"
	case AlertTokenSurge:
		return "token_surge"
	default:
		return "unknown"
	}
}

// AlertEntry is stored in the alert_history table.
type AlertEntry struct {
	ID        int64     `json:"id"`
	Type      AlertType `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// MetricsHistoryRow is stored in the metrics_history table.
type MetricsHistoryRow struct {
	ID            int64     `json:"id"`
	ProviderName  string    `json:"provider_name"`
	Balance       float64   `json:"balance"`
	Currency      string    `json:"currency"`
	AccountStatus string    `json:"account_status"`
	TodayTokens   int64     `json:"today_tokens"`
	MonthTokens   int64     `json:"month_tokens"`
	CollectedAt   time.Time `json:"collected_at"`
}
