package context

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/brandur/modulr/log"
	"github.com/brandur/modulr/parallel"
	"github.com/fsnotify/fsnotify"
)

// Args are the set of arguments accepted by NewContext.
type Args struct {
	Concurrency int
	Log         log.LoggerInterface
	Pool        *parallel.Pool
	Port        string
	SourceDir   string
	TargetDir   string
	Watcher     *fsnotify.Watcher
}

// Context contains useful state that can be used by a user-provided build
// function.
type Context struct {
	// Concurrency is the number of concurrent workers to run during the build
	// step.
	Concurrency int

	// FirstRun indicates whether this is the first run of the build loop.
	FirstRun bool

	// Jobs is a channel over which jobs to be done are transmitted.
	Jobs chan parallel.Job

	// Log is a logger that can be used to print information.
	Log log.LoggerInterface

	// Port specifies the port on which to serve content from TargetDir over
	// HTTP.
	Port string

	// SourceDir is the directory containing source files.
	SourceDir string

	// Stats tracks various statistics about the build process.
	//
	// Statistics are reset between build loops, but are cumulative between
	// build phases within a loop (i.e. calls to Wait).
	Stats *Stats

	// TargetDir is the directory where the site will be built to.
	TargetDir string

	// Watcher is a file system watcher that picks up changes to source files
	// and restarts the build loop.
	Watcher *fsnotify.Watcher

	// fileModTimeCache remembers the last modified times of files.
	fileModTimeCache *FileModTimeCache

	// forced indicates whether change checking should be bypassed.
	forced bool

	// mu is a mutex used to synchronize access on watchedPaths.
	mu *sync.Mutex

	// pool is the job pool used to build the static site.
	pool *parallel.Pool

	// watchedPaths keeps track of what paths we're currently watching.
	watchedPaths map[string]struct{}
}

// NewContext initializes and returns a new Context.
func NewContext(args *Args) *Context {
	return &Context{
		Concurrency: args.Concurrency,
		FirstRun:    true,
		Jobs:        args.Pool.JobsChan,
		Log:         args.Log,
		Port:        args.Port,
		SourceDir:   args.SourceDir,
		Stats:       &Stats{},
		TargetDir:   args.TargetDir,
		Watcher:     args.Watcher,

		fileModTimeCache: NewFileModTimeCache(args.Log),
		mu:               new(sync.Mutex),
		pool:             args.Pool,
		watchedPaths:     make(map[string]struct{}),
	}
}

// AddJob is a shortcut for adding a new job to the Jobs channel.
func (c *Context) AddJob(name string, f func() (bool, error)) {
	c.Jobs <- parallel.Job {Name: name, F: f}
}

// Changed returns whether the target path's modified time has changed since
// the last time it was checked. It also saves the last modified time for
// future checks.
//
// TODO: It also makes sure the root path is being watched.
func (c *Context) Changed(path string) bool {
	if !c.exists(path) {
		return false
	}

	// Normalize the path (Abs also calls Clean).
	path, err := filepath.Abs(path)
	if err != nil {
		c.Log.Errorf("Error normalizing path: %v", err)
	}

	err = c.addWatched(path)
	if err != nil {
		c.Log.Errorf("Error watching source: %v", err)
	}

	return c.fileModTimeCache.changed(path)
}

// ChangedAny is the same as Changed except it returns true if any of the given
// paths have changed.
func (c *Context) ChangedAny(paths []string) bool {
	for _, path := range paths {
		changed := c.Changed(path)
		if changed {
			return true
		}
	}
	return false
}

// Forced returns whether change checking is disabled in the current context.
//
// Functions using a forced context still return the right value for their
// unchanged return, but execute all their work.
//
// TODO: Rename to IsForced to match IsUnchanged.
func (c *Context) Forced() bool {
	return c.forced
}

// ForcedContext returns a copy of the current Context for which change
// checking is disabled.
//
// Functions using a forced context still return the right value for their
// unchanged return, but execute all their work.
func (c *Context) ForcedContext() *Context {
	forceC := c.clone()
	forceC.forced = true
	return forceC
}

// Wait waits on the job pool to execute its current round of jobs.
//
// Returns true if the round of jobs all executed successfully, and false
// otherwise. In the latter case, a work function should return so that the
// modulr main loop can print the errors that occurred.
//
// If all jobs were successful, the worker pool is restarted so that more jobs
// can be queued. If it wasn't, the jobs channel will be closed, and trying to
// enqueue a new one will panic.
func (c *Context) Wait() bool {
	// Wait for work to finish.
	c.pool.Wait()

	c.Stats.JobsExecuted = append(c.Stats.JobsExecuted, c.pool.JobsExecuted...)
	c.Stats.NumJobs += c.pool.NumJobs
	c.Stats.NumJobsExecuted += c.pool.NumJobsExecuted

	if c.pool.Errors != nil {
		return false
	}

	// Then start the pool again, which also has the side effect of
	// reinitializing anything that needs to be reinitialized.
	c.pool.Run()

	// This channel is reinitialized, so make sure to pull in the new one.
	c.Jobs = c.pool.JobsChan

	return true
}

func (c *Context) addWatched(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Watch the parent directory unless the file is a directory itself. This
	// will hopefully mean fewer individual entries in the notifier.
	if !info.IsDir() {
		path = filepath.Dir(path)
	}

	// Do nothing if we're already watching the path.
	_, ok := c.watchedPaths[path]
	if ok {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Watcher.Add(path)
}

// clone clones the current Context.
func (c *Context) clone() *Context {
	return &Context{
		Concurrency: c.Concurrency,
		Log:         c.Log,
		SourceDir:   c.SourceDir,
		Stats:       c.Stats,
		TargetDir:   c.TargetDir,
		Watcher:     c.Watcher,

		fileModTimeCache: c.fileModTimeCache,
		forced:           c.forced,
		mu:               c.mu,
		watchedPaths:     c.watchedPaths,
	}
}

func (c *Context) exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		c.Log.Errorf("Error checking file existence: %v", err)
	}
	return false
}

// FileModTimeCache tracks the last modified time of files seen so a
// determination can be made as to whether they need to be recompiled.
type FileModTimeCache struct {
	log              log.LoggerInterface
	mu               sync.Mutex
	pathToModTimeMap map[string]time.Time
}

// NewFileModTimeCache returns a new FileModTimeCache.
func NewFileModTimeCache(log log.LoggerInterface) *FileModTimeCache {
	return &FileModTimeCache{
		log:              log,
		pathToModTimeMap: make(map[string]time.Time),
	}
}

// changed returns whether the target path's modified time has changed since
// the last time it was checked. It also saves the last modified time for
// future checks.
func (c *FileModTimeCache) changed(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			c.log.Errorf("Error stat'ing file: %v", err)
		}
		return true
	}

	modTime := stat.ModTime()

	c.mu.Lock()
	lastModTime, ok := c.pathToModTimeMap[path]
	c.pathToModTimeMap[path] = modTime
	c.mu.Unlock()

	if !ok {
		return true
	}

	changed := lastModTime.Before(modTime)
	if !changed {
		c.log.Debugf("context: No changes to source: %s", path)
		return false
	}

	// TODO: Debug help. Remove eventually.
	c.log.Infof("context: File did change: %s (last mod time = %v, mod time = %v", path, lastModTime, modTime)

	return true
}

// Stats tracks various statistics about the build process.
type Stats struct {
	// JobsExecuted is a slice of jobs that were executed on the last run.
	JobsExecuted []*parallel.Job

	// NumJobs is the total number of jobs generated for the build loop.
	NumJobs int64

	// NumJobsExecuted is the number of jobs that did some kind of heavier
	// lifting during the build loop. That's those that returned `true` on
	// execution.
	NumJobsExecuted int64

	// Start is the start time of the build loop.
	Start time.Time
}

// Reset resets statistics.
func (s *Stats) Reset() {
	s.JobsExecuted = nil
	s.NumJobs = 0
	s.NumJobsExecuted = 0
	s.Start = time.Now()
}
