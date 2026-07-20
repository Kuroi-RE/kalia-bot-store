// Package worker runs background jobs (reconciliation, cleanup, delivery retry)
// on fixed cadences. It is executed by the cmd/worker binary, separate from the
// API server so jobs scale and restart independently.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// JobFunc is a unit of background work. It returns a human-readable summary and
// an error; both are logged by the scheduler.
type JobFunc func(ctx context.Context) (summary string, err error)

type job struct {
	name     string
	interval time.Duration
	run      JobFunc
}

// Scheduler runs registered jobs on tickers until its context is cancelled.
type Scheduler struct {
	log  *slog.Logger
	jobs []job
}

// NewScheduler builds a scheduler.
func NewScheduler(log *slog.Logger) *Scheduler {
	return &Scheduler{log: log}
}

// Register adds a job to run every interval.
func (s *Scheduler) Register(name string, interval time.Duration, fn JobFunc) {
	s.jobs = append(s.jobs, job{name: name, interval: interval, run: fn})
}

// Run starts all jobs and blocks until ctx is cancelled. Each job runs once
// shortly after start, then on its interval.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, j := range s.jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			s.exec(ctx, j) // initial immediate run
			ticker := time.NewTicker(j.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.exec(ctx, j)
				}
			}
		}(j)
	}
	s.log.Info("worker scheduler started", slog.Int("jobs", len(s.jobs)))
	wg.Wait()
	s.log.Info("worker scheduler stopped")
}

func (s *Scheduler) exec(ctx context.Context, j job) {
	start := time.Now()
	summary, err := j.run(ctx)
	if err != nil {
		s.log.Error("job failed", slog.String("job", j.name), slog.Any("error", err), slog.Duration("took", time.Since(start)))
		return
	}
	if summary != "" {
		s.log.Info("job ran", slog.String("job", j.name), slog.String("summary", summary), slog.Duration("took", time.Since(start)))
	}
}
