package modulir

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestEmptyPool(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 10)
	p.Init()
	defer p.Stop()

	p.StartRound()
	p.Wait()

	assert.Equal(t, []error(nil), p.Errors)
}

func TestWithWork(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 10)
	fmt.Printf("init\n")
	p.Init()
	defer p.Stop()

	p.StartRound()
	p.Jobs <- NewJob("job 0", func() (bool, error) { return true, nil })
	p.Jobs <- NewJob("job 1", func() (bool, error) { return true, nil })
	p.Jobs <- NewJob("job 2", func() (bool, error) { return false, nil })
	p.Wait()

	assert.Equal(t, []error(nil), p.Errors)
	assert.Equal(t, int64(3), p.NumJobs)
	assert.Equal(t, 2, len(p.JobsExecuted)) // Number of `return true` above
}

func TestWithError(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 10)
	fmt.Printf("init\n")
	p.Init()
	defer p.Stop()

	p.StartRound()
	p.Jobs <- NewJob("job 0", func() (bool, error) { return true, nil })
	p.Jobs <- NewJob("job 1", func() (bool, error) { return true, nil })
	p.Jobs <- NewJob("job 2", func() (bool, error) { return true, fmt.Errorf("error") })
	p.Wait()

	assert.Equal(t, []error{fmt.Errorf("error")}, p.Errors)
	assert.Equal(t, int64(3), p.NumJobs)
	assert.Equal(t, 3, len(p.JobsExecuted)) // Number of `return true` above
}
