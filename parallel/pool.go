package parallel

import (
	"sync"

	"github.com/brandur/modulr/log"
)

// Pool is a worker group that runs a number of jobs at a configured
// concurrency.
type Pool struct {
	Errors []error
	JobsChan chan func() error

	// NumJobs is the number of jobs that went through a work iteration of the
	// pool.
	//
	// This number is not accurate until Wait has finished fully. It's reset
	// when Run is called.
	NumJobs int

	concurrency int

	// Send errors through a channel to make appending to `errors` Goroutine
	// safe.
	errorsChan chan error

	errorsFeederDone chan bool
	jobsChanInternal chan func() error
	jobsFeederDone chan bool
	log log.LoggerInterface
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
	p.JobsChan = make(chan func() error, 100)
	p.NumJobs = 0
	p.errorsChan = make(chan error)
	p.errorsFeederDone = make(chan bool)
	p.jobsChanInternal = make(chan func() error, 100)
	p.jobsFeederDone = make(chan bool)
	p.running = true

	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}

	// Error feeder
	go func() {
		for err := range p.errorsChan {
			p.Errors = append(p.Errors, err)
		}

		// Runs after errorsChan has been closed.
		p.errorsFeederDone <- true
	}()

	// Job feeder
	go func() {
		for job := range p.JobsChan {
			p.NumJobs++
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

	// Signal to the errors channel that all work is done.
	close(p.errorsChan)

	// Now wait for the error feeder to be finished so that we know all errors
	// have been pushed to Errors.
	<- p.errorsFeederDone

	if p.Errors != nil {
		return false
	}
	return true
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for job := range p.jobsChanInternal {
		err := job()
		if err != nil {
			p.errorsChan <- err
		}

		p.wg.Done()
	}
}
