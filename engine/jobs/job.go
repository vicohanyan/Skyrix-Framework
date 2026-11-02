package jobs

import kerneljobs "gitlab.com/skyrix-lib/framework/kernel/jobs"

// Compatibility aliases keep the existing engine/jobs API while ownership of
// the contracts lives in kernel/jobs.
type Job = kerneljobs.Job
type Registry = kerneljobs.Registry
