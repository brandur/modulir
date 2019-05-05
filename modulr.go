package modulr

import (
	"net/http"
	"os"
	"path"
	"time"

	"github.com/brandur/modulr/context"
	"github.com/brandur/modulr/log"
	"github.com/brandur/modulr/mod/mfile"
	"github.com/brandur/modulr/parallel"
	"github.com/fsnotify/fsnotify"
)

// Config contains configuration.
type Config struct {
	// Concurrency is the number of concurrent workers to run during the build
	// step.
	//
	// Defaults to 10.
	Concurrency int

	// Log specifies a logger to use.
	//
	// Defaults to an instance of Logger running at informational level.
	Log log.LoggerInterface

	// Port specifies the port on which to serve content from TargetDir over
	// HTTP.
	//
	// Defaults to not running if left empty.
	Port string

	// SourceDir is the directory containing source files.
	//
	// Defaults to ".".
	SourceDir string

	// TargetDir is the directory where the site will be built to.
	//
	// Defaults to "./public".
	TargetDir string
}

// Context contains useful state that can be used by a user-provided build
// function.
type Context = context.Context

// Build is one of the main entry points to the program. Call this to build
// only one time.
func Build(config *Config, f func(*context.Context) error) {
	build(config, f, false)
}

// BuildLoop is one of the main entry points to the program. Call this to build
// in a perpetual loop.
func BuildLoop(config *Config, f func(*context.Context) error) {
	build(config, f, true)
}

//
// Private
//

func build(config *Config, f func(*context.Context) error, loop bool) {
	var errors []error

	if config == nil {
		config = &Config{}
	}

	fillDefaults(config)

	pool := parallel.NewPool(config.Log, config.Concurrency)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		config.Log.Errorf("Error starting watcher: %v", err)
		os.Exit(1)
	}
	defer watcher.Close()

	c := context.NewContext(&context.Args{
		Log:       config.Log,
		Port:      config.Port,
		Pool:      pool,
		SourceDir: config.SourceDir,
		TargetDir: config.TargetDir,
		Watcher:   watcher,
	})

	rebuild := make(chan struct{})
	rebuildDone := make(chan struct{})
	go watchChanges(c, watcher, rebuild, rebuildDone)

	startServer := make(chan struct{})
	go func() {
		<-startServer
		serveHTTP(c)
	}()

	for {
		c.Log.Debugf("Start loop")
		c.Stats.Reset()

		pool.Run()
		c.Jobs = pool.JobsChan

		err := mfile.EnsureDir(c, c.TargetDir)
		if err != nil {
			goto wait
		}

		err = f(c)

	wait:
		c.Wait()

		errors = pool.Errors
		if err != nil {
			errors = append([]error{err}, errors...)
		}

		if errors != nil {
			for i, err := range errors {
				c.Log.Errorf("Build error: %v", err)

				if i >= 9 {
					c.Log.Errorf("Too many errors.")
					break
				}
			}
		}

		c.Log.Infof("Built site in %s (%v / %v job(s) did work)",
			time.Now().Sub(c.Stats.Start), c.Stats.NumJobsExecuted, c.Stats.NumJobs)

		if !loop {
			break
		}

		if c.FirstRun {
			startServer <- struct{}{}
			c.FirstRun = false
		} else {
			rebuildDone <- struct{}{}
		}

		<-rebuild
	}

	if errors != nil {
		os.Exit(1)
	}
}

func fillDefaults(config *Config) {
	if config.Concurrency <= 0 {
		config.Concurrency = 10
	}

	if config.Log == nil {
		config.Log = &log.Logger{Level: log.LevelInfo}
	}

	if config.SourceDir == "" {
		config.SourceDir = "."
	}

	if config.TargetDir == "" {
		config.TargetDir = "./public"
	}
}

func serveHTTP(c *context.Context) {
	c.Log.Infof("Serving '%s' on port %s", path.Clean(c.TargetDir), c.Port)
	c.Log.Infof("Open browser to: http://localhost:%s/", c.Port)
	handler := http.FileServer(http.Dir(c.TargetDir))
	err := http.ListenAndServe(":"+c.Port, handler)
	if err != nil {
		c.Log.Errorf("Error starting server: %v", err)
		os.Exit(1)
	}
}

func shouldRebuild(op fsnotify.Op) bool {
	if op&fsnotify.Chmod == fsnotify.Chmod {
		return false
	}

	return true
}

func watchChanges(c *context.Context, watcher *fsnotify.Watcher, rebuild, rebuildDone chan struct{}) {
OUTER:
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if !shouldRebuild(event.Op) {
				continue
			}

			c.Log.Infof("Detected change; rebuilding")

			// Start rebuild
			rebuild <- struct{}{}

			// Wait until rebuild is finished. In the meantime, drain any
			// new events that come in on the watcher's channel.
			for {
				select {
				case <-rebuildDone:
					continue OUTER
				case <-watcher.Events:
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			c.Log.Errorf("Error from watcher:", err)
		}
	}
}
