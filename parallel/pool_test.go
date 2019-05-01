package parallel

import (
	"fmt"
	"testing"

	"github.com/brandur/modulr/log"
	assert "github.com/stretchr/testify/require"
)

func TestEmptyPool(t *testing.T) {
	p := NewPool(&log.Logger{Level: log.LevelDebug}, 10)
	p.Run()

	p.Wait()

	assert.Equal(t, []error(nil), p.Errors)
}

func TestWithWork(t *testing.T) {
	p := NewPool(&log.Logger{Level: log.LevelDebug}, 10)
	p.Run()

	p.JobsChan <- func() (bool, error) { return true, nil }
	p.JobsChan <- func() (bool, error) { return true, nil }
	p.JobsChan <- func() (bool, error) { return false, nil }
	p.Wait()

	assert.Equal(t, []error(nil), p.Errors)
	assert.Equal(t, int64(3), p.NumJobs)
	assert.Equal(t, int64(2), p.NumJobsExecuted) // Number of `return true` above
}

func TestWithError(t *testing.T) {
	p := NewPool(&log.Logger{Level: log.LevelDebug}, 10)
	p.Run()

	p.JobsChan <- func() (bool, error) { return true, nil }
	p.JobsChan <- func() (bool, error) { return true, nil }
	p.JobsChan <- func() (bool, error) { return true, fmt.Errorf("error") }
	p.Wait()

	assert.Equal(t, []error{fmt.Errorf("error")}, p.Errors)
	assert.Equal(t, int64(3), p.NumJobs)
	assert.Equal(t, int64(3), p.NumJobsExecuted) // Number of `return true` above
}
