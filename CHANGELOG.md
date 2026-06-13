# Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] — 2026-06-13

### Changed

- Repository renamed to `aimon-min` (极简AI监控)

### Added

- **DeepSeek Balance Monitoring** — Real-time display of DeepSeek account balance on a desktop widget, updated every 60s (configurable)
- **Desktop Widget** — Lightweight Win32 overlay showing provider name and balance at a glance
  - Draggable, semi-transparent, no taskbar icon
  - Auto-saves position across restarts
  - Rounded corners
- **System Tray** — Tray icon with quick-access menu showing:
  - Current balance
  - Today's token usage
  - Month's token usage
  - Refresh, Settings, Auto-start toggle, Exit actions
- **Auto-start on Boot** — Toggle from tray menu to register/unregister Windows startup entry
- **Windows Toast Notifications** — Alerts for:
  - Low balance (below configurable threshold)
  - API errors
  - Token usage surge (above configurable % of daily average)
  - 30-minute cooldown per notification type to prevent spam
- **SQLite Storage** — Persistent storage for metrics history and alert tracking
- **Configurable via YAML** — API key, notification thresholds, refresh interval, proxy settings

### Technical

- Native Win32 API for desktop widget (no WebView, no Fyne)
- Go 1.24, CGo-free widget rendering (Win32 GDI)
- Single binary deployment (~12 MB), no runtime dependencies
- Graceful shutdown with goroutine lifecycle management
- Memory target: <30MB idle
