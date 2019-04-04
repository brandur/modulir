package context

import (
	"os"
	"time"

	"github.com/brandur/modulr/log"
	"github.com/brandur/modulr/parallel"
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
	Jobs chan parallel.Job

	// Log is a logger that can be used to print information.
	Log log.LoggerInterface

	// SourceDir is the directory containing source files.
	SourceDir string

	// TargetDir is the directory where the site will be built to.
	TargetDir string
}

// HasChanged returns whether the target path's modified time has changed since
// the last time it was checked. It also saves the last modified time for
// future checks.
//
// TODO: It also makes sure the root path is being watched.
func (c *Context) HasChanged(path string) bool {
	return c.FileModTimeCache.hasChanged(path)
}

// FileModTimeCache tracks the last modified time of files seen so a
// determination can be made as to whether they need to be recompiled.
type FileModTimeCache struct {
	log log.LoggerInterface
	pathToModTimeMap map[string]time.Time
}

// NewFileModTimeCache returns a new FileModTimeCache.
func NewFileModTimeCache(log log.LoggerInterface) *FileModTimeCache {
	return &FileModTimeCache{
		log: log,
		pathToModTimeMap: make(map[string]time.Time),
	}
}

// hasChanged returns whether the target path's modified time has changed since
// the last time it was checked. It also saves the last modified time for
// future checks.
func (c *FileModTimeCache) hasChanged(path string) bool {
	stat, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		c.log.Errorf("Error stat'ing file: %v", err)
		return true
	}

	modTime := stat.ModTime()
	lastModTime, ok := c.pathToModTimeMap[path]
	c.pathToModTimeMap[path] = modTime

	if !ok {
		return true
	}

	changed := lastModTime.Before(modTime)
	c.log.Debugf("No changes to source: %s", path)

	return changed
}
