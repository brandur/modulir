package parallel

import (
	"sync"

	"github.com/brandur/modulr/log"
)

// Pool is a worker group that runs a number of jobs at a configured
// concurrency.
type Pool struct {
	Errors []error
	JobsChan chan Job

	concurrency int

	// Send errors through a channel to make appending to `errors` Goroutine
	// safe.
	errorsChan chan error

	errorsFeederDone chan bool
	jobsChanInternal chan Job
	jobsFeederDone chan bool
	log log.LoggerInterface
	numJobs int
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

	p.JobsChan = make(chan Job, 100)
	p.errorsChan = make(chan error)
	p.errorsFeederDone = make(chan bool)
	p.jobsChanInternal = make(chan Job, 100)
	p.jobsFeederDone = make(chan bool)

	p.numJobs = 0

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
			p.jobsChanInternal <- job
			p.numJobs++
			p.wg.Add(1)
		}

		// Runs after JobsChan has been closed.
		p.jobsFeederDone <- true
	}()
}

// Wait waits until all jobs are finished and stops the pool.
func (p *Pool) Wait() {
	// First signal over the jobs chan that all work has been enqueued).
	close(p.JobsChan)

	// Now wait for the job feeder to be finished so that we know all jobs have
	// been enqueued in jobsChanInternal.
	<- p.jobsFeederDone

	p.log.Debugf("Waiting for %v job(s) to be done", p.numJobs)

	// Now wait for all those jobs to be done.
	p.wg.Wait()

	// close channel to stop workers
	close(p.jobsChanInternal)

	// Signal to the errors channel that all work is done.
	close(p.errorsChan)

	// Now wait for the error feeder to be finished so that we know all errors
	// have been pushed to Errors.
	<- p.errorsFeederDone
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for job := range p.jobsChanInternal {
		err := job.Work()
		if err != nil {
			p.errorsChan <- err
		}

		p.wg.Done()
	}
}
