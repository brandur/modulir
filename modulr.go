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

	c := context.NewContext(&context.Args{
		Log:       config.Log,
		Port:      config.Port,
		Pool:      pool,
		SourceDir: config.SourceDir,
		TargetDir: config.TargetDir,
	})

	startServer := make(chan struct{})
	go func() {
		<- startServer

		c.Log.Infof("Serving '%s' on port %s", path.Clean(c.TargetDir), c.Port)
		c.Log.Infof("Open browser to: http://localhost:%s/", c.Port)
		handler := http.FileServer(http.Dir(c.TargetDir))
		err := http.ListenAndServe(":"+c.Port, handler)
		if err != nil {
			c.Log.Errorf("Error starting server: %v", err)
			os.Exit(1)
		}
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
		}

		// TODO: Change to watch file system changes instead.
		time.Sleep(60 * time.Second)
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
