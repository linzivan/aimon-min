package tray

import (
	"fmt"
	"sync"

	"ai-monitor/internal/autostart"
	"ai-monitor/internal/icon"
	"ai-monitor/internal/types"

	"github.com/getlantern/systray"
)

// MenuState carries the latest metrics for display in the menu.
type MenuState struct {
	mu      sync.RWMutex
	metrics *types.Metrics
	err     error
}

// Tray manages the system tray icon and menu.
type Tray struct {
	state     *MenuState
	onRefresh func()
	onExit    func()
	onSetting func()

	// Menu items
	balanceItem    *systray.MenuItem
	todayItem      *systray.MenuItem
	monthItem      *systray.MenuItem
	refreshItem    *systray.MenuItem
	settingItem    *systray.MenuItem
	autoStartItem  *systray.MenuItem
	exitItem       *systray.MenuItem
}

// New creates a tray manager.
func New(onRefresh, onExit, onSetting func()) *Tray {
	return &Tray{
		state:     &MenuState{},
		onRefresh: onRefresh,
		onExit:    onExit,
		onSetting: onSetting,
	}
}

// Update refreshes the displayed metrics.
func (t *Tray) Update(m *types.Metrics, err error) {
	t.state.mu.Lock()
	t.state.metrics = m
	t.state.err = err
	t.state.mu.Unlock()

	t.refreshMenu()
}

// Start runs the system tray (blocking).
func (t *Tray) Start() {
	systray.Run(t.onReady, t.onExit_)
}

// Stop the tray.
func (t *Tray) Stop() {
	systray.Quit()
}

func (t *Tray) onReady() {
	systray.SetTooltip("AI Monitor")
	systray.SetIcon(icon.Data)

	// Use a simple icon (16x16 green dot in ICO format would be ideal)
	// For now, set a minimal icon via icon bytes
	// We use a simple approach - set icon data

	t.balanceItem = systray.AddMenuItem("DeepSeek ....", "View balance")
	t.todayItem = systray.AddMenuItem("Today ....", "Today's token usage")
	t.monthItem = systray.AddMenuItem("Month ....", "Month's token usage")

	systray.AddSeparator()

	t.refreshItem = systray.AddMenuItem("Refresh", "Refresh metrics now")
	t.settingItem = systray.AddMenuItem("Settings", "Open settings (NYI)")

	systray.AddSeparator()

	t.autoStartItem = systray.AddMenuItemCheckbox("Auto-start on boot", "Start AI Monitor when Windows starts", false)
	// Sync checkbox with actual registry state
	if enabled, err := autostart.IsEnabled(); err == nil && enabled {
		t.autoStartItem.Check()
	}

	systray.AddSeparator()

	t.exitItem = systray.AddMenuItem("Exit", "Exit AI Monitor")

	// Event handlers
	go func() {
		for {
			select {
			case <-t.refreshItem.ClickedCh:
				if t.onRefresh != nil {
					t.onRefresh()
				}
			case <-t.settingItem.ClickedCh:
				if t.onSetting != nil {
					t.onSetting()
				}
			case <-t.autoStartItem.ClickedCh:
				if t.autoStartItem.Checked() {
					autostart.Disable()
					t.autoStartItem.Uncheck()
				} else {
					autostart.Enable()
					t.autoStartItem.Check()
				}
			case <-t.exitItem.ClickedCh:
				if t.onExit != nil {
					t.onExit()
				}
			}
		}
	}()

	fmt.Println("[tray] system tray ready")
}

func (t *Tray) onExit_() {
	// Cleanup if needed
}

func (t *Tray) refreshMenu() {
	t.state.mu.RLock()
	defer t.state.mu.RUnlock()

	if t.balanceItem == nil {
		return
	}

	if t.state.err != nil {
		t.balanceItem.SetTitle("DeepSeek Error")
		t.todayItem.SetTitle("Error fetching data")
		t.monthItem.SetTitle("")
		return
	}

	m := t.state.metrics
	if m == nil {
		t.balanceItem.SetTitle("DeepSeek ....")
		t.todayItem.SetTitle("Today ....")
		t.monthItem.SetTitle("Month ....")
		return
	}

	if m.Error != "" {
		t.balanceItem.SetTitle(fmt.Sprintf("DeepSeek \u00a5%.2f (API error)", m.Balance))
		t.todayItem.SetTitle("Today ...")
		t.monthItem.SetTitle("Month ...")
		return
	}

	t.balanceItem.SetTitle(fmt.Sprintf("DeepSeek \u00a5%.2f", m.Balance))
	t.todayItem.SetTitle(fmt.Sprintf("Today %s Token", formatTokens(m.TodayTokens)))
	t.monthItem.SetTitle(fmt.Sprintf("Month %s Token", formatTokens(m.MonthTokens)))
}

func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
