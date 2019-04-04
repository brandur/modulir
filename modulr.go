package modulr

import (
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
	// Defaults to 5.
	Concurrency int

	// Log specifies a logger to use.
	//
	// Defaults to an instance of Logger running at informational level.
	Log log.LoggerInterface

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

// BuildLoop is the entrypoint to the program.
func BuildLoop(config *Config, f func(*context.Context) error) {
	if config == nil {
		config = &Config{}
	}

	fillDefaults(config)

	c := &context.Context{
		FileModTimeCache: context.NewFileModTimeCache(config.Log),
		Log:              config.Log,
		TargetDir:        config.TargetDir,
	}

	pool := parallel.NewPool(config.Log, config.Concurrency)

	for {
		c.Log.Infof("Start loop")
		start := time.Now()

		pool.Run()
		c.Jobs = pool.JobsChan

		err := mfile.EnsureDir(c, c.TargetDir)
		if err != nil {
			goto wait
		}

		err = f(c)

	wait:
		pool.Wait()

		errors := pool.Errors
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

		c.Log.Infof("Built site in %s", time.Now().Sub(start))

		// TODO: Change to watch file system changes instead.
		time.Sleep(60 * time.Second)
	}
}

func Job(f func() error) parallel.Job {
	return &parallel.GenericJob{F: f}
}

func fillDefaults(config *Config) {
	if config.Concurrency <= 0 {
		config.Concurrency = 5
	}

	if config.Log == nil {
		config.Log = &log.Logger{Level: log.LevelDebug}
	}

	if config.SourceDir == "" {
		config.SourceDir = "."
	}

	if config.TargetDir == "" {
		config.TargetDir = "./public"
	}
}
