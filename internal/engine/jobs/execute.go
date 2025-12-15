package jobs

import (
	"context"
	"fmt"
	"skyrix/internal/logger"
	"time"
)

// ExecuteJob runs a job synchronously with retry/backoff and panic protection.
// Retries are limited by Job.RetryCount() with a linear backoff (100ms * attempt).
func ExecuteJob(ctx context.Context, job Job, log logger.Interface, args map[string]any) (err error) {
	if job == nil {
		if log != nil {
			log.Error("job is nil")
		}
		return fmt.Errorf("job is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job %q panicked: %v", job.Name(), r)
			if log != nil {
				log.Error("job panicked", "name", job.Name())
			}
		}
	}()

	maxRetries := job.RetryCount()
	if maxRetries < 0 {
		maxRetries = 0
	}

	var attempt int
	for {
		attempt++
		err = job.Execute(ctx, args)
		if err == nil {
			return nil
		}

		if attempt > maxRetries {
			log.Error(fmt.Sprintf("job %q failed after %d retries", job.Name(), attempt), "name", job.Name())
			return fmt.Errorf("job %q failed after %d attempts: %w", job.Name(), attempt, err)
		}

		time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
	}
}

// ExecuteJobAsync spawns ExecuteJob in a goroutine and emits lifecycle logs.
func ExecuteJobAsync(ctx context.Context, job Job, log logger.Interface, args map[string]any) {
	if job == nil {
		if log != nil {
			log.Error("async job is nil")
		}
		return
	}

	go func() {
		if log != nil {
			log.Info("async job started", "job", job.Name())
		}

		if err := ExecuteJob(ctx, job, log, args); err != nil {
			if log != nil {
				log.Error("async job failed", "job", job.Name(), "error", err)
			}
			return
		}

		if log != nil {
			log.Info("async job finished", "job", job.Name())
		}
	}()
}
