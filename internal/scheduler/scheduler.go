package scheduler

import (
	"context"

	"ai-monitor/internal/logger"
	"sync"
	"time"
)

// Task represents a scheduled recurring task.
type Task struct {
	Name     string
	Interval time.Duration
	Handler  func(ctx context.Context) error
}

// Scheduler is a unified scheduler that manages all periodic tasks.
// No other component should create its own tickers or goroutines.
// All periodic work MUST be registered here.
type Scheduler struct {
	mu      sync.Mutex
	tasks   []*Task
	cancels []context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// New creates a new Scheduler.
func New() *Scheduler {
	return &Scheduler{}
}

// Register adds a task to the scheduler.
// Must be called before Start() or while stopped.
func (s *Scheduler) Register(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append(s.tasks, task)
	logger.Info("[scheduler] registered task: %s (every %v)", task.Name, task.Interval)
}

// Start begins all registered tasks.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}
	s.running = true

	logger.Info("[scheduler] starting %d tasks...", len(s.tasks))

	for _, task := range s.tasks {
		taskCtx, cancel := context.WithCancel(ctx)
		s.cancels = append(s.cancels, cancel)

		s.wg.Add(1)
		go s.runTask(taskCtx, task)
	}
}

// Stop cancels all running tasks and waits for them to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false

	logger.Info("[scheduler] stopping all tasks...")
	for _, cancel := range s.cancels {
		cancel()
	}
	s.cancels = nil
	s.wg.Wait()
	logger.Info("[scheduler] all tasks stopped")
}

// Running returns whether the scheduler is running.
func (s *Scheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Scheduler) runTask(ctx context.Context, task *Task) {
	defer s.wg.Done()

	// Run immediately on start
	s.executeTask(ctx, task)

	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.executeTask(ctx, task)
		}
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task *Task) {
	// Recover from any panic so the goroutine lives on
	defer func() {
		if r := recover(); r != nil {
			logger.Error("[scheduler] task %s PANICKED: %v", task.Name, r)
		}
	}()

	// If context is already done, skip
	if ctx.Err() != nil {
		return
	}

	start := time.Now()
	err := task.Handler(ctx)
	elapsed := time.Since(start)

	if err != nil {
		logger.Error("[scheduler] task %s failed after %v: %v", task.Name, elapsed, err)
	} else {
		logger.Info("[scheduler] task %s completed in %v", task.Name, elapsed)
	}
}
