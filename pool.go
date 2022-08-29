package modulir

import (
	"errors"
	"sort"
	"sync"
	"time"

	"golang.org/x/xerrors"
)

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Public
//
//
//
//////////////////////////////////////////////////////////////////////////////

// Job is a wrapper for a piece of work that should be executed by the job
// pool.
//
//nolint:errname
type Job struct {
	// Duration is the time it took the job to run. It's set regardless of
	// whether the job's finished state was executed, not executed, or errored.
	Duration time.Duration

	// Err is an error that the job produced, if any.
	Err error

	// Executed is whether the job "did work", signaled by it returning true.
	Executed bool

	// F is the function which makes up the job's workload.
	F func() (bool, error)

	// Name is a name for the job which is helpful for informational and
	// debugging purposes.
	Name string
}

// Error returns the error message of the error wrapped in the job if this was
// an errored job. Job implements the error interface so that it can return
// itself in situations where error handling is being done but job errors may
// be mixed in with other sorts of errors.
//
// It panics if the job wasn't errored, so be careful to only use this when
// iterating across something like Pool.JobsErrored.
func (j *Job) Error() string {
	if j.Err == nil {
		panic("Error called on a non-errored Job")
	}

	return j.Err.Error()
}

// NewJob initializes and returns a new Job.
func NewJob(name string, f func() (bool, error)) *Job {
	return &Job{Name: name, F: f}
}

// Pool is a worker group that runs a number of jobs at a configured
// concurrency.
type Pool struct {
	Jobs chan *Job

	// JobsAll is a slice of all the jobs that were fed into the pool on the
	// last run.
	JobsAll []*Job

	// JobsErrored is a slice of jobs that errored on the last run.
	//
	// See also JobErrors which is a shortcut for extracting all the errors
	// from the jobs.
	JobsErrored []*Job

	// JobsExecuted is a slice of jobs that were executed on the last run.
	JobsExecuted []*Job

	colorizer      *colorizer
	concurrency    int
	jobsInternal   chan *Job
	jobsErroredMu  sync.Mutex
	jobsExecutedMu sync.Mutex
	jobsFeederDone chan struct{}
	log            LoggerInterface
	roundNum       int
	roundStarted   bool
	wg             sync.WaitGroup
	workerInfos    []workerInfo
}

// NewPool initializes a new pool with the given jobs and at the given
// concurrency. It calls Init so that the pool is fully spun up and ready to
// start a round.
func NewPool(log LoggerInterface, concurrency int) *Pool {
	// By default a pool gets a no-op colorizer. NewContext may set one
	// separately for pools created within the package.
	pool := &Pool{
		colorizer:   &colorizer{LogColor: false},
		concurrency: concurrency,
		log:         log,
		workerInfos: make([]workerInfo, concurrency),
	}
	return pool
}

// JobErrors is a shortcut from extracting all the errors out of JobsErrored,
// the set of jobs that errored on the last round.
func (p *Pool) JobErrors() []error {
	if len(p.JobsErrored) < 1 {
		return nil
	}

	errs := make([]error, len(p.JobsErrored))
	for i, job := range p.JobsErrored {
		errs[i] = job.Err
	}
	return errs
}

// LogErrors logs a limited set of errors that occurred during a build.
func (p *Pool) LogErrors() {
	p.LogErrorsSlice(p.JobErrors())
}

// LogErrorsSlice logs a limited set of errors from the given slice.
func (p *Pool) LogErrorsSlice(errs []error) {
	if errs == nil {
		return
	}

	for i, err := range errs {
		// When dealing with an errored job (in practice, this is going to be
		// the common case), we can provide a little more detail on what went
		// wrong.
		var job *Job
		if errors.As(err, &job) {
			p.log.Errorf(
				p.colorizer.Bold(p.colorizer.Red("Job error:")).String()+
					" %v (job: '%s', time: %v)",
				job.Err, job.Name, job.Duration.Truncate(100*time.Microsecond))
		} else {
			p.log.Errorf(
				p.colorizer.Bold(p.colorizer.Red("Build error:")).String()+
					" %v",
				err)
		}

		if i >= maxMessages-1 {
			p.log.Errorf("... too many errors (limit reached)")
			break
		}
	}
}

// LogSlowest logs a limited set of executed jobs from the last build starting
// with the slowest jobs on top.
func (p *Pool) LogSlowest() {
	p.LogSlowestSlice(p.JobsExecuted)
}

// LogSlowestSlice logs a limited set of executed jobs from the given slice.
func (p *Pool) LogSlowestSlice(jobs []*Job) {
	sortJobsBySlowest(jobs)

	for i, job := range jobs {
		// Having this in the loop ensures we don't print it if zero jobs
		// executed
		if i == 0 {
			p.log.Infof("Jobs executed (slowest first):")
		}

		p.log.Infof(
			p.colorizer.Bold(p.colorizer.Cyan("    %s")).String()+
				" (time: %v)",
			job.Name, job.Duration.Truncate(100*time.Microsecond))

		if i >= maxMessages-1 {
			p.log.Infof("... many jobs executed (limit reached)")
			break
		}
	}
}

// StartRound begins an execution round. Internal statistics and other tracking
// are all reset.
func (p *Pool) StartRound(roundNum int) {
	if p.roundStarted {
		panic("StartRound already called (call Wait before calling it again)")
	}

	p.roundNum = roundNum
	p.log.Debugf("pool: Starting round %v at concurrency %v", p.roundNum, p.concurrency)

	p.Jobs = make(chan *Job, 500)
	p.JobsAll = nil
	p.JobsErrored = nil
	p.JobsExecuted = nil
	p.jobsFeederDone = make(chan struct{}, 1)
	p.jobsInternal = make(chan *Job, 500)
	p.roundStarted = true

	for i := range p.workerInfos {
		p.workerInfos[i].reset()
	}

	// Job feeder
	go func() {
		p.log.Debugf("pool: Job feeder: Starting")

		for job := range p.Jobs {
			p.wg.Add(1)
			p.jobsInternal <- job
			p.JobsAll = append(p.JobsAll, job)
		}

		p.log.Debugf("pool: Job feeder: Finished feeding")

		// Runs after Jobs has been closed.
		close(p.jobsFeederDone)
	}()

	// Worker Goroutines
	for i := 0; i < p.concurrency; i++ {
		workerNum := i
		go func() {
			p.workForRound(workerNum)
		}()
	}
}

// Wait waits until all jobs are finished and stops the pool.
//
// Returns true if the round of jobs all executed successfully, and false
// otherwise. In the latter case, the caller should stop and observe the
// contents of Errors.
//
// If the pool isn't running, it falls through without doing anything so it's
// safe to call Wait multiple times.
func (p *Pool) Wait() bool {
	if !p.roundStarted {
		panic("Can't wait on a job pool that's not primed (call StartRound first)")
	}

	p.log.Debugf("pool: Starting round %v", p.roundNum)

	p.roundStarted = false

	// First signal over the jobs chan that all work has been enqueued).
	close(p.Jobs)

	// Now wait for the job feeder to be finished so that we know all jobs have
	// been enqueued in jobsInternal.
	<-p.jobsFeederDone

	// Prints some debug information to help us in case we run into stalling
	// problems in the main job loop.
	done := make(chan struct{}, 1)
	go func() {
		select {
		case <-time.After(waitSoftTimeout):
			p.logWaitTimeoutInfo()
		case <-done:
		}
	}()

	p.log.Debugf("pool: Waiting for %v job(s) to be done", len(p.JobsAll))

	// Now wait for all those jobs to be done.
	p.wg.Wait()

	// Kill the timeout Goroutine.
	done <- struct{}{}

	// Drops workers out of their run loop. Their Goroutines return.
	// wait on the run gate.
	close(p.jobsInternal)

	// Occasionally useful for debugging.
	// p.logWaitTimeoutInfo()

	return p.JobsErrored == nil
}

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Private
//
//
//
//////////////////////////////////////////////////////////////////////////////

const (
	// When to report that a job is probably timed out. We call it a "soft"
	// timeout because we can't actually kill jobs.
	jobSoftTimeout = 15 * time.Second

	// Maximum number of errors or jobs to print on screen after a build loop.
	maxMessages = 10

	// When to report that a wait round is probably timed out. We call it a
	// "soft" timeout because no jobs are killed -- it's just for reporting and
	// debugging purposes.
	waitSoftTimeout = 60 * time.Second
)

// Keeps track of the information on a worker. Used for debugging purposes
// only.
type workerInfo struct {
	activeJob *Job
	state     workerState

	// Number of jobs finished is the total number that the worker "saw" as it
	// was running this round, so it includes errored jobs as well as jobs that
	// didn't "execute", which means that they were given a chance to run, but
	// decided no changes were required and therefore didn't do any serious
	// work.
	numJobsErrored  int
	numJobsExecuted int
	numJobsFinished int
}

// Resets statistics for the worker info.
func (wi *workerInfo) reset() {
	wi.state = workerStateWaitingOnRunOrStop
	wi.numJobsErrored = 0
	wi.numJobsExecuted = 0
	wi.numJobsFinished = 0
}

// Keeps track of the state of a worker. Used for debugging purposes only.
type workerState string

// The possible states that a worker can be in. Used for debugging purposes
// only.
const (
	workerStateJobExecuting       workerState = "job_executing"
	workerStateJobFinished        workerState = "job_finished"
	workerStatePanicked           workerState = "panicked"
	workerStateStopped            workerState = "stopped"
	workerStateWaitingOnRunOrStop workerState = "waiting_on_run_or_stop"
)

func (p *Pool) logWaitTimeoutInfo() {
	// We don't have an easy channel to count on for this number, so sum the
	// numbers across all workers.
	numJobsFinished := 0
	for _, info := range p.workerInfos {
		numJobsFinished += info.numJobsFinished
	}

	p.log.Errorf(
		"Wait soft timeout (jobs queued: %v, finished: %v, errored: %v, executed: %v, left: %v)",
		len(p.JobsAll),
		numJobsFinished,
		len(p.JobsErrored),
		len(p.JobsExecuted),
		len(p.jobsInternal),
	)

	for i, info := range p.workerInfos {
		jobName := "<none>"
		if info.activeJob != nil {
			jobName = info.activeJob.Name
		}

		p.log.Errorf("    Worker %v state: %v, jobs finished: %v, errored: %v, executed: %v, job: %v",
			i, info.state, info.numJobsFinished, info.numJobsErrored, info.numJobsExecuted, jobName)
	}
}

// Puts a finished job in the right channel and adds run statistics to the
// worker's info.
func (p *Pool) setWorkerJobFinished(workerNum int, job *Job, executed bool, err error) {
	p.workerInfos[workerNum].numJobsFinished++

	if err != nil {
		job.Err = err

		p.jobsErroredMu.Lock()
		p.JobsErrored = append(p.JobsErrored, job)
		p.jobsErroredMu.Unlock()

		p.workerInfos[workerNum].numJobsErrored++
	}

	if executed {
		job.Executed = true

		p.jobsExecutedMu.Lock()
		p.JobsExecuted = append(p.JobsExecuted, job)
		p.jobsExecutedMu.Unlock()

		p.workerInfos[workerNum].numJobsExecuted++
	}

	p.wg.Done()

	p.workerInfos[workerNum].activeJob = nil
	p.workerInfos[workerNum].state = workerStateJobFinished
}

func (p *Pool) setWorkerJobExecuting(workerNum int, job *Job) {
	p.workerInfos[workerNum].activeJob = job
	p.workerInfos[workerNum].state = workerStateJobExecuting
}

// Sorts a slice of jobs with the slowest on top.
func sortJobsBySlowest(jobs []*Job) {
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[j].Duration < jobs[i].Duration
	})
}

// The work loop for a single round within a single worker Goroutine.
func (p *Pool) workForRound(workerNum int) {
	for j := range p.jobsInternal {
		// Required so that we have a stable pointer that we can keep past the
		// lifetime of the loop. Don't change this.
		job := j

		p.workJob(workerNum, job)
	}

	p.workerInfos[workerNum].state = workerStateStopped
}

// A worker working a single job. Extracted this way so that we can add a defer
// that will help debug a panic.
func (p *Pool) workJob(workerNum int, job *Job) {
	p.setWorkerJobExecuting(workerNum, job)

	// Start a Goroutine to track the time taken to do this work.
	// Unfortunately, we can't actually kill a timed out Goroutine because
	// Go (and we rely on the user to make sure these get fixed instead),
	// but we can at least raise on the interface which job is problematic
	// to help identify what needs to be fixed.
	done := make(chan struct{}, 1)
	go func() {
		select {
		case <-time.After(jobSoftTimeout):
			p.log.Errorf("Job soft timeout (job: '%s')", job.Name)
		case <-done:
		}
	}()

	var executed bool
	var jobErr error
	start := time.Now()

	defer func() {
		job.Duration = time.Since(start)

		// Kill the timeout Goroutine.
		done <- struct{}{}

		var panicked bool
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				jobErr = xerrors.Errorf("job panicked: %w", err)
			} else {
				// Panics are often just given a string to panic with, so make
				// sure to handle that as well
				jobErr = xerrors.Errorf("job panicked: %v", r)
			}
			panicked = true
		}

		p.setWorkerJobFinished(workerNum, job, executed, jobErr)

		// And set the special panicked worker status if we panicked
		// because it means that this worker is down and no longer
		// available.
		//
		// TODO: It is possible to hit a deadlock if all workers have
		// panicked and there's still work left to do. The framework should
		// detect this condition and exit.
		if panicked {
			p.workerInfos[workerNum].state = workerStatePanicked
		}
	}()

	executed, jobErr = job.F()
}
