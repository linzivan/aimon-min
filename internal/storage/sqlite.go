package storage

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"ai-monitor/internal/proxy"
	"ai-monitor/internal/types"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db  *sql.DB
	mu  sync.Mutex
	dsn string
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &Store{db: db, dsn: dsn}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	fmt.Printf("[storage] opened %s\n", dsn)
	return s, nil
}

func (s *Store) migrate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS metrics_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			balance REAL NOT NULL DEFAULT 0,
			currency TEXT NOT NULL DEFAULT 'CNY',
			account_status TEXT NOT NULL DEFAULT 'active',
			today_tokens INTEGER NOT NULL DEFAULT 0,
			month_tokens INTEGER NOT NULL DEFAULT 0,
			collected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_collected ON metrics_history(collected_at)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_provider ON metrics_history(provider_name, collected_at)`,
		`CREATE TABLE IF NOT EXISTS alert_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_type_time ON alert_history(type, created_at)`,
		`CREATE TABLE IF NOT EXISTS token_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			model TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_token_usage_created ON token_usage(created_at)`,
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) SaveMetrics(m *types.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO metrics_history (provider_name, balance, currency, account_status, today_tokens, month_tokens, collected_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ProviderName, m.Balance, m.Currency, m.AccountStatus, m.TodayTokens, m.MonthTokens, m.CollectedAt,
	)
	return err
}

func (s *Store) GetLatestMetrics() (*types.Metrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT provider_name, balance, currency, account_status, today_tokens, month_tokens, collected_at FROM metrics_history ORDER BY collected_at DESC LIMIT 1`)
	m := &types.Metrics{}
	err := row.Scan(&m.ProviderName, &m.Balance, &m.Currency, &m.AccountStatus, &m.TodayTokens, &m.MonthTokens, &m.CollectedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (s *Store) SaveAlert(a *types.AlertEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO alert_history (type, message, created_at) VALUES (?, ?, ?)`, a.Type.String(), a.Message, a.CreatedAt)
	return err
}

func (s *Store) HasRecentAlert(alertType types.AlertType, cooldown time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	since := time.Now().Add(-cooldown)
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM alert_history WHERE type = ? AND created_at >= ?`, alertType.String(), since).Scan(&count)
	return count > 0, err
}

func (s *Store) GetConfig(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var value string
	err := s.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetConfig(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, key, value)
	return err
}

func (s *Store) CleanupOldData(before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM metrics_history WHERE collected_at < ?`, before)
	tx.Exec(`DELETE FROM alert_history WHERE created_at < ?`, before)
	return tx.Commit()
}

func (s *Store) SaveTokenRecord(r *proxy.TokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO token_usage (prompt_tokens, completion_tokens, total_tokens, model, created_at) VALUES (?, ?, ?, ?, ?)`,
		r.PromptTokens, r.CompletionTokens, r.TotalTokens, r.Model, r.CreatedAt,
	)
	return err
}

func (s *Store) GetTokenUsage(since time.Time) (prompt, completion, total int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(
		`SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(total_tokens),0) FROM token_usage WHERE created_at >= ?`,
		since,
	)
	err = row.Scan(&prompt, &completion, &total)
	return
}

func (s *Store) Close() error {
	fmt.Println("[storage] closing...")
	return s.db.Close()
}
