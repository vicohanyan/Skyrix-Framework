package providers

import (
	"testing"

	"skyrix/internal/jobs"
	kernelJobs "skyrix/internal/kernel/jobs"
)

func TestProvideRegisteredJobsRegistryRegistersAllJobs(t *testing.T) {
	reg := kernelJobs.NewRegistry(noopProviderLogger{})
	registered := ProvideRegisteredJobsRegistry(reg, &Jobs{
		SystemPingJob: jobs.NewSystemPingJob(noopProviderLogger{}),
	})

	for _, name := range []string{
		"system.ping",
	} {
		if _, ok := registered.Get(name); !ok {
			t.Fatalf("registered.Get(%q) ok = false, want true", name)
		}
	}
}

type noopProviderLogger struct{}

func (noopProviderLogger) Error(msg string, keysAndValues ...interface{}) {}
func (noopProviderLogger) Info(msg string, keysAndValues ...interface{})  {}
func (noopProviderLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (noopProviderLogger) Warn(msg string, keysAndValues ...interface{})  {}
