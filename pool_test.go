package modulir

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"golang.org/x/xerrors"
)

func TestEmptyPool(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 10)

	p.StartRound(0)
	p.Wait()

	assert.Equal(t, 0, len(p.JobsAll))
	assert.Equal(t, 0, len(p.JobsErrored))
	assert.Equal(t, 0, len(p.JobsExecuted))
	assert.Equal(t, []error(nil), p.JobErrors())
}

func TestWithWork(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 10)

	p.StartRound(0)
	j0 := NewJob("job 0", func() (bool, error) { return true, nil })
	p.Jobs <- j0
	j1 := NewJob("job 1", func() (bool, error) { return true, nil })
	p.Jobs <- j1
	j2 := NewJob("job 2", func() (bool, error) { return false, nil })
	p.Jobs <- j2
	p.Wait()

	// Check state on the pool
	assert.Equal(t, 3, len(p.JobsAll))
	assert.Equal(t, 0, len(p.JobsErrored))
	assert.Equal(t, 2, len(p.JobsExecuted)) // Number of `return true` above
	assert.Equal(t, []error(nil), p.JobErrors())

	// Check state on individual jobs
	assert.Equal(t, true, j0.Executed)
	assert.Equal(t, nil, j0.Err)
	assert.Equal(t, true, j1.Executed)
	assert.Equal(t, nil, j1.Err)
	assert.Equal(t, false, j2.Executed)
	assert.Equal(t, nil, j2.Err)
}

// Tests the pool with lots of fast jobs that do nothing across multiple
// rounds. Originally written to try to suss out a race condition.
func TestWithLargeNonWork(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 30)

	numJobs := 300
	numRounds := 50

	for i := 0; i < numRounds; i++ {
		p.StartRound(0)
		for j := 0; j < numJobs; j++ {
			p.Jobs <- NewJob("job", func() (bool, error) { return false, nil })
		}
		p.Wait()

		// Check state on the pool
		assert.Equal(t, numJobs, len(p.JobsAll))
		assert.Equal(t, 0, len(p.JobsErrored))
		assert.Equal(t, 0, len(p.JobsExecuted)) // Number of `return true` above
		assert.Equal(t, []error(nil), p.JobErrors())
	}
}

func TestWithError(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 10)

	p.StartRound(0)
	j0 := NewJob("job 0", func() (bool, error) { return true, nil })
	p.Jobs <- j0
	j1 := NewJob("job 1", func() (bool, error) { return true, nil })
	p.Jobs <- j1
	j2 := NewJob("job 2", func() (bool, error) { return true, xerrors.Errorf("error") })
	p.Jobs <- j2
	p.Wait()

	// Check state on the pool
	assert.Equal(t, 3, len(p.JobsAll))
	assert.Equal(t, 1, len(p.JobsErrored))
	assert.Equal(t, 3, len(p.JobsExecuted)) // Number of `return true` above
	assert.Equal(t, []string{"error"}, errorStrings(p.JobErrors()))

	// Check state on individual jobs
	assert.Equal(t, true, j0.Executed)
	assert.Equal(t, nil, j0.Err)
	assert.Equal(t, true, j1.Executed)
	assert.Equal(t, nil, j1.Err)
	assert.Equal(t, true, j2.Executed)
	assert.Equal(t, "error", j2.Err.Error())
}

func TestWorkJob(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 1)

	executed := false
	j := &Job{
		F: func() (bool, error) {
			executed = true
			return true, nil
		},
		Name: "TestJob",
	}

	p.wg.Add(1)
	p.workJob(0, j)

	assert.True(t, executed)

	assert.Equal(t, 0, len(p.JobsErrored))
	assert.Equal(t, 1, len(p.JobsExecuted))

	assert.Equal(t, true, j.Executed)
	assert.Equal(t, nil, j.Err)
}

func TestWorkJob_Error(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 1)

	executed := false
	j := &Job{
		F: func() (bool, error) {
			executed = true
			return true, xerrors.Errorf("error")
		},
		Name: "TestJob",
	}

	p.wg.Add(1)
	p.workJob(0, j)

	assert.True(t, executed)

	assert.Equal(t, 1, len(p.JobsErrored))
	assert.Equal(t, 1, len(p.JobsExecuted))
	assert.Equal(t, []string{"error"}, errorStrings(p.JobErrors()))

	assert.Equal(t, true, j.Executed)
	assert.Equal(t, "error", j.Err.Error())
}

func TestWorkJob_Panic(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 1)

	executed := false
	j := &Job{
		F: func() (bool, error) {
			executed = true
			panic(xerrors.Errorf("error"))
		},
		Name: "TestJob",
	}

	p.wg.Add(1)
	p.workJob(0, j)

	assert.True(t, executed)

	assert.Equal(t, 1, len(p.JobsErrored))
	assert.Equal(t, 0, len(p.JobsExecuted))

	err := p.JobErrors()[0]
	assert.Equal(t, "job panicked: error", err.Error())

	assert.Equal(t, false, j.Executed)
	assert.Equal(t, "job panicked: error", j.Err.Error())
}

func TestWorkJob_PanicString(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 1)

	executed := false
	j := &Job{
		F: func() (bool, error) {
			executed = true
			panic("error")
		},
		Name: "TestJob",
	}

	p.wg.Add(1)
	p.workJob(0, j)

	assert.True(t, executed)

	assert.Equal(t, 1, len(p.JobsErrored))
	assert.Equal(t, 0, len(p.JobsExecuted))
	assert.Equal(t, []string{"job panicked: error"}, errorStrings(p.JobErrors()))

	assert.Equal(t, false, j.Executed)
	assert.Equal(t, "job panicked: error", j.Err.Error())
}

func errorStrings(errs []error) []string {
	strs := make([]string, len(errs))
	for i, err := range errs {
		strs[i] = err.Error()
	}
	return strs
}
