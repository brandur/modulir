package parallel

import (
)

// Job is an interface for work to be done.
type Job interface {
	// Work works the job.
	Work() error
}

// GenericJob is a generic job that uses a closure as its work body.
type GenericJob struct {
	F func() error
}

// Work works the job.
func (j *GenericJob) Work() error {
	return j.F()
}
