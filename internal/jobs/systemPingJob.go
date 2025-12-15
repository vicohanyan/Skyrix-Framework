package jobs

import (
	"context"

	engineJobs "skyrix/internal/engine/jobs"
	"skyrix/internal/logger"
)

// SystemPingJob is a safe, dependency-free framework job.
// It is intended for smoke-testing the jobs subsystem (DI + registry + runner).
type SystemPingJob struct {
	Log logger.Interface
}

func NewSystemPingJob(log logger.Interface) *SystemPingJob {
	return &SystemPingJob{Log: log}
}

func (j *SystemPingJob) Name() string { return "system.ping" }

func (j *SystemPingJob) RetryCount() int { return 0 }

// Execute logs a ping and returns nil.
// Expected args (optional):
//   - "msg": string
func (j *SystemPingJob) Execute(ctx context.Context, args map[string]any) error {
	_ = ctx

	msg, _ := args["msg"].(string)
	if msg == "" {
		msg = "ok"
	}

	return nil
}

var _ engineJobs.Job = (*SystemPingJob)(nil)
