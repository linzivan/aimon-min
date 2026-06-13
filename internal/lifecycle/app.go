package lifecycle

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"ai-monitor/internal/autostart"
	"ai-monitor/internal/config"
	"ai-monitor/internal/notification"
	"ai-monitor/internal/provider"
	"ai-monitor/internal/proxy"
	"ai-monitor/internal/scheduler"
	"ai-monitor/internal/storage"
	"ai-monitor/internal/tray"
	"ai-monitor/internal/types"
	"ai-monitor/internal/widget"
)

// App is the central application lifecycle manager.
// It owns all components and manages their Start/Stop/Shutdown lifecycle.
type App struct {
	cfg    *config.Config
	prx    *proxy.Proxy
	store  *storage.Store
	sched  *scheduler.Scheduler
	notify *notification.Notifier
	widget *widget.Widget
	tray   *tray.Tray
	p      *provider.DeepSeekProvider

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex

	lastMetrics *types.Metrics
}

// New creates a fully initialized application.
func New(cfg *config.Config) (*App, error) {
	// Initialize storage
	store, err := storage.New(cfg.DBPath())
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}

	// Initialize scheduler
	sched := scheduler.New()

	// Initialize notification manager
	notify := notification.New(cfg.Notifications.CooldownMinutes)

	// Initialize DeepSeek provider
	p := provider.NewDeepSeek(cfg.DeepSeek.APIKey, cfg.DeepSeek.BaseURL)

	// Widget disabled for V1 - Win32 GDI painting needs stabilization
	// Initialize proxy (if enabled)
	var prx *proxy.Proxy
	if cfg.Proxy.Enabled {
		prx = proxy.New(cfg.DeepSeek.BaseURL, cfg.DeepSeek.APIKey, store)
		fmt.Printf("[app] proxy enabled on %s\n", cfg.Proxy.Listen)
	}

	// Initialize widget (v2 - static controls)
	wdgt := widget.New(store)

	app := &App{
		prx: prx,
		cfg:    cfg,
		store:  store,
		sched:  sched,
		notify: notify,
		widget: wdgt,
		p:      p,
	}

	// Register tasks
	refreshInterval := time.Duration(cfg.Monitor.RefreshInterval) * time.Second
	if refreshInterval < 10*time.Second {
		refreshInterval = 10 * time.Second
	}

	sched.Register(&scheduler.Task{
		Name:     "collect_metrics",
		Interval: refreshInterval,
		Handler:  app.collectMetrics,
	})

	// Create tray callbacks
	app.tray = tray.New(
		app.onRefresh, // refresh callback
		app.onExit,    // exit callback
		app.onSetting, // settings callback
	)

	// Initial data load
	if latest, err := store.GetLatestMetrics(); err == nil {
		app.lastMetrics = latest
	}

	return app, nil
}

// Run starts all components and blocks until shutdown.
// This is the main entry point for the application.
func (a *App) Run() error {
	a.mu.Lock()
	if a.ctx != nil {
		a.mu.Unlock()
		return fmt.Errorf("already running")
	}
	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.mu.Unlock()

	fmt.Println("[app] starting AI Monitor...")

	// Start scheduler (collects metrics on a timer)
	a.sched.Start(a.ctx)

	// Start proxy (if enabled)
	if a.prx != nil {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			if err := a.prx.Start(a.cfg.Proxy.Listen); err != nil {
				fmt.Printf("[app] proxy error: %v\n", err)
			}
		}()
		time.Sleep(50 * time.Millisecond) // let proxy bind
	}

	// Apply auto-start setting from config
	if a.cfg.General.AutoStart {
		autostart.Enable()
	}

	// Start widget in background
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.widget.Start(); err != nil {
			fmt.Printf("[app] widget error: %v\n", err)
		}
	}()

	time.Sleep(200 * time.Millisecond)

	// Initial update
	a.refreshDisplay()

	// Run tray (blocking)
	fmt.Println("[app] tray running (blocking)...")
	a.tray.Start()

	return nil
}

// Shutdown gracefully stops all components.
func (a *App) Shutdown() {
	fmt.Println("[app] shutting down...")

	// Stop scheduler first (no more data collection)
	a.sched.Stop()

	// Stop proxy
	if a.prx != nil {
		a.prx.Stop()
	}

	// Stop widget
	a.widget.Stop()

	// Stop tray
	a.tray.Stop()

	// Cancel context
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()

	// Wait for goroutines
	a.wg.Wait()

	// Close provider
	a.p.Close()

	// Close storage last
	if a.store != nil {
		a.store.Close()
	}

	fmt.Println("[app] shutdown complete")
}

// collectMetrics is the main task handler called by the scheduler.
func (a *App) collectMetrics(ctx context.Context) error {
	if a.cfg.DeepSeek.APIKey == "" {
		return fmt.Errorf("DeepSeek API key not configured")
	}

	metrics, err := a.p.Collect(ctx)
	if err != nil {
		fmt.Printf("[app] collect error: %v\n", err)

		// Send notification if API error (with cooldown)
		if a.notify != nil {
			a.notify.SendAPIError("DeepSeek", err)
		}

		// Update display with error state
		m := &types.Metrics{
			ProviderName: "deepseek",
			Error:        err.Error(),
			CollectedAt:  time.Now(),
		}
		a.lastMetrics = m
		a.refreshDisplay()
		return err
	}

	// Save to database
	if a.store != nil {
		a.store.SaveMetrics(metrics)
	}

	// Update token stats from proxy (if enabled)
	if a.prx != nil {
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

		if prompt, completion, total, err := a.store.GetTokenUsage(todayStart); err == nil {
			metrics.TodayTokens = total
			_ = prompt
			_ = completion
		}
		if _, _, monthTotal, err := a.store.GetTokenUsage(monthStart); err == nil {
			metrics.MonthTokens = monthTotal
		}
	}

	// Check thresholds and send notifications
	a.checkNotifications(metrics)

	// Update display
	a.lastMetrics = metrics
	a.refreshDisplay()
	return nil
}

// checkNotifications evaluates metrics against thresholds.
func (a *App) checkNotifications(m *types.Metrics) {
	if a.notify == nil || a.store == nil {
		return
	}

	// Balance low check
	if m.Balance < a.cfg.Notifications.BalanceThreshold && m.Balance > 0 {
		cooldown := time.Duration(a.cfg.Notifications.CooldownMinutes) * time.Minute
		recent, err := a.store.HasRecentAlert(types.AlertBalanceLow, cooldown)
		if err == nil && !recent {
			a.notify.SendBalanceLow(m.Balance, a.cfg.Notifications.BalanceThreshold)
			a.store.SaveAlert(&types.AlertEntry{
				Type:      types.AlertBalanceLow,
				Message:   fmt.Sprintf("Balance low: %.2f", m.Balance),
				CreatedAt: time.Now(),
			})
		}
	}

	// Token surge check (compare today with monthly average)
	if m.MonthTokens > 0 && a.lastMetrics != nil {
		daysInMonth := time.Now().Day()
		if daysInMonth > 0 {
			dailyAvg := float64(m.MonthTokens) / float64(daysInMonth)
			if dailyAvg > 0 {
				surgePercent := (float64(m.TodayTokens) - dailyAvg) / dailyAvg * 100
				if surgePercent > float64(a.cfg.Notifications.TokenSurgeThreshold) {
					cooldown := time.Duration(a.cfg.Notifications.CooldownMinutes) * time.Minute
					recent, err := a.store.HasRecentAlert(types.AlertTokenSurge, cooldown)
					if err == nil && !recent {
						a.notify.SendTokenSurge(m.TodayTokens, surgePercent)
						a.store.SaveAlert(&types.AlertEntry{
							Type:      types.AlertTokenSurge,
							Message:   fmt.Sprintf("Token surge: %.0f%% above avg", surgePercent),
							CreatedAt: time.Now(),
						})
					}
				}
			}
		}
	}
}

// refreshDisplay updates the widget and tray with latest metrics.
func (a *App) refreshDisplay() {
	if a.widget != nil {
		a.widget.Update(a.lastMetrics)
	}
	a.tray.Update(a.lastMetrics, nil)
}

// onRefresh is called when the user clicks "Refresh" in the tray.
func (a *App) onRefresh() {
	fmt.Println("[app] manual refresh requested")
	// Run collection synchronously
	err := a.collectMetrics(context.Background())
	if err != nil {
		fmt.Printf("[app] refresh failed: %v\n", err)
	}
}

// onExit is called when the user clicks "Exit".
func (a *App) onExit() {
	fmt.Println("[app] exit requested")
	// Shutdown may not fully unwind because systray.Quit() posts to the
	// main thread's message queue from a different goroutine.
	// Force process exit to guarantee clean termination.
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	a.Shutdown()
	os.Exit(0)
}

// onSetting is called when the user clicks "Settings".
func (a *App) onSetting() {
	fmt.Println("[app] settings requested (NYI)")
	a.notify.SendBalanceLow(0, 0) // Placeholder
}
