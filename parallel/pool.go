package parallel

import (
	"sync"
	"sync/atomic"

	"github.com/brandur/modulr/log"
)

// Job is a wrapper for a piece of work that should be executed by the job
// pool.
type Job struct {
	F func() (bool, error)
	Name string
}

// Pool is a worker group that runs a number of jobs at a configured
// concurrency.
type Pool struct {
	Errors []error
	JobsChan chan Job

	// JobsExecuted is a slice of jobs that were executed on the last run.
	JobsExecuted []*Job

	// NumJobs is the number of jobs that went through a work iteration of the
	// pool.
	//
	// This number is not accurate until Wait has finished fully. It's reset
	// when Run is called.
	NumJobs int64

	// NumJobsExecuted is the number of jobs that did some kind of heavier
	// lifting during the build loop. That's those that returned `true` on
	// execution.
	//
	// This number is not accurate until Wait has finished fully. It's reset
	// when Run is called.
	NumJobsExecuted int64

	concurrency int

	jobsChanInternal chan Job
	jobsFeederDone chan bool
	log log.LoggerInterface
	mu sync.Mutex
	running bool
	wg          sync.WaitGroup
}

// NewPool initializes a new pool with the given jobs and at the given
// concurrency.
func NewPool(log log.LoggerInterface, concurrency int) *Pool {
	return &Pool{
		concurrency: concurrency,
		log: log,
	}
}

// Run spings up workers starts working jobs.
func (p *Pool) Run() {
	p.log.Debugf("Running job pool at concurrency %v", p.concurrency)

	p.Errors = nil
	p.JobsChan = make(chan Job, 500)
	p.JobsExecuted = nil
	p.NumJobs = 0
	p.NumJobsExecuted = 0
	p.jobsChanInternal = make(chan Job, 500)
	p.jobsFeederDone = make(chan bool)
	p.running = true

	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}

	// Job feeder
	go func() {
		for job := range p.JobsChan {
			p.jobsChanInternal <- job
			p.wg.Add(1)
		}

		// Runs after JobsChan has been closed.
		p.jobsFeederDone <- true
	}()
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
	if !p.running {
		return true
	}

	p.running = false

	// First signal over the jobs chan that all work has been enqueued).
	close(p.JobsChan)

	// Now wait for the job feeder to be finished so that we know all jobs have
	// been enqueued in jobsChanInternal.
	<- p.jobsFeederDone

	p.log.Debugf("pool: Waiting for %v job(s) to be done", p.NumJobs)

	// Now wait for all those jobs to be done.
	p.wg.Wait()

	// close channel to stop workers
	close(p.jobsChanInternal)

	if p.Errors != nil {
		return false
	}
	return true
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for job := range p.jobsChanInternal {
		executed, err := job.F()
		if err != nil {
			p.mu.Lock()
			p.Errors = append(p.Errors, err)
			p.mu.Unlock()
		}

		atomic.AddInt64(&p.NumJobs, 1)
		if executed {
			atomic.AddInt64(&p.NumJobsExecuted, 1)
			
			p.mu.Lock()
			p.JobsExecuted = append(p.JobsExecuted, &job)
			p.mu.Unlock()
		}

		p.wg.Done()
	}
}
