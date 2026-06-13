package notification

import (
	"fmt"
	"sync"
	"time"

	"ai-monitor/internal/types"

	"github.com/gen2brain/beeep"
)

// Notifier handles Windows Toast notifications with cooldown deduplication.
// Cooldown prevents the same notification type from firing too frequently.
type Notifier struct {
	mu        sync.Mutex
	cooldown  time.Duration
	lastSent  map[types.AlertType]time.Time
}

// New creates a notification manager.
func New(cooldownMinutes int) *Notifier {
	if cooldownMinutes <= 0 {
		cooldownMinutes = 30
	}
	return &Notifier{
		cooldown: time.Duration(cooldownMinutes) * time.Minute,
		lastSent: make(map[types.AlertType]time.Time),
	}
}

// SendBalanceLow alerts when balance is below threshold.
func (n *Notifier) SendBalanceLow(balance float64, threshold float64) error {
	msg := fmt.Sprintf("DeepSeek balance low: ¥%.2f (threshold: ¥%.2f)", balance, threshold)
	return n.send(types.AlertBalanceLow, "AI Monitor - Balance Low", msg)
}

// SendAPIError alerts when an API call fails.
func (n *Notifier) SendAPIError(provider string, err error) error {
	msg := fmt.Sprintf("%s API error: %v", provider, err)
	return n.send(types.AlertAPIError, "AI Monitor - API Error", msg)
}

// SendTokenSurge alerts on unusual token consumption.
func (n *Notifier) SendTokenSurge(todayTokens int64, percentIncrease float64) error {
	msg := fmt.Sprintf("Today token usage: %d (%.0f%% above average)", todayTokens, percentIncrease)
	return n.send(types.AlertTokenSurge, "AI Monitor - Token Surge", msg)
}

// send fires a notification if the cooldown period has passed.
func (n *Notifier) send(alertType types.AlertType, title, message string) error {
	n.mu.Lock()
	last, exists := n.lastSent[alertType]
	now := time.Now()
	if exists && now.Sub(last) < n.cooldown {
		n.mu.Unlock()
		return nil // Cooldown active, skip
	}
	n.lastSent[alertType] = now
	n.mu.Unlock()

	fmt.Printf("[notify] %s: %s\n", title, message)
	return beeep.Alert(title, message, "")
}
