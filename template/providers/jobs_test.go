package providers

import (
	"testing"

	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
	"gitlab.com/skyrix-lib/template/internal/jobs"
)

func TestProvideRegisteredJobsRegistryRegistersAllJobs(t *testing.T) {
	reg := engineJobs.NewRegistry(noopProviderLogger{})
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
