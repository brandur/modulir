package context

import (
	"os"
	"sync"
	"time"

	"github.com/brandur/modulr/log"
)

// Context contains useful state that can be used by a user-provided build
// function.
type Context struct {
	// Concurrency is the number of concurrent workers to run during the build
	// step.
	Concurrency int

	// FileModTimeCache remembers the last modified times of files.
	FileModTimeCache *FileModTimeCache

	// Jobs is a channel over which jobs to be done are transmitted.
	Jobs chan func() error

	// Log is a logger that can be used to print information.
	Log log.LoggerInterface

	// SourceDir is the directory containing source files.
	SourceDir string

	// TargetDir is the directory where the site will be built to.
	TargetDir string
}

// IsUnchanged returns whether the target path's modified time has changed since
// the last time it was checked. It also saves the last modified time for
// future checks.
//
// TODO: It also makes sure the root path is being watched.
func (c *Context) IsUnchanged(path string) bool {
	return c.FileModTimeCache.isUnchanged(path)
}

// FileModTimeCache tracks the last modified time of files seen so a
// determination can be made as to whether they need to be recompiled.
type FileModTimeCache struct {
	log log.LoggerInterface
	mu sync.Mutex
	pathToModTimeMap map[string]time.Time
}

// NewFileModTimeCache returns a new FileModTimeCache.
func NewFileModTimeCache(log log.LoggerInterface) *FileModTimeCache {
	return &FileModTimeCache{
		log: log,
		pathToModTimeMap: make(map[string]time.Time),
	}
}

// isUnchanged returns whether the target path's modified time has changed since
// the last time it was checked. It also saves the last modified time for
// future checks.
func (c *FileModTimeCache) isUnchanged(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
	    if !os.IsNotExist(err) {
			c.log.Errorf("Error stat'ing file: %v", err)
		}
		return false
	}

	modTime := stat.ModTime()

	c.mu.Lock()
	lastModTime, ok := c.pathToModTimeMap[path]
	c.pathToModTimeMap[path] = modTime
	c.mu.Unlock()

	if !ok {
		return false
	}

	changed := lastModTime.Before(modTime)
	if !changed {
		c.log.Debugf("No changes to source: %s", path)
	}

	return !changed
}
