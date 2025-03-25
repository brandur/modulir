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

	assert.Empty(t, p.JobsAll)
	assert.Empty(t, p.JobsErrored)
	assert.Empty(t, p.JobsExecuted)
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
	assert.Len(t, p.JobsAll, 3)
	assert.Empty(t, p.JobsErrored)
	assert.Len(t, p.JobsExecuted, 2) // Number of `return true` above
	assert.Equal(t, []error(nil), p.JobErrors())

	// Check state on individual jobs
	assert.True(t, j0.Executed)
	assert.NoError(t, j0.Err)
	assert.True(t, j1.Executed)
	assert.NoError(t, j1.Err)
	assert.False(t, j2.Executed)
	assert.NoError(t, j2.Err)
}

// Tests the pool with lots of fast jobs that do nothing across multiple
// rounds. Originally written to try to suss out a race condition.
func TestWithLargeNonWork(t *testing.T) {
	p := NewPool(&Logger{Level: LevelDebug}, 30)

	numJobs := 300
	numRounds := 50

	for range numRounds {
		p.StartRound(0)
		for range numJobs {
			p.Jobs <- NewJob("job", func() (bool, error) { return false, nil })
		}
		p.Wait()

		// Check state on the pool
		assert.Len(t, p.JobsAll, numJobs)
		assert.Empty(t, p.JobsErrored)
		assert.Empty(t, p.JobsExecuted) // Number of `return true` above
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
	assert.Len(t, p.JobsAll, 3)
	assert.Len(t, p.JobsErrored, 1)
	assert.Len(t, p.JobsExecuted, 3) // Number of `return true` above
	assert.Equal(t, []string{"error"}, errorStrings(p.JobErrors()))

	// Check state on individual jobs
	assert.True(t, j0.Executed)
	assert.NoError(t, j0.Err)
	assert.True(t, j1.Executed)
	assert.NoError(t, j1.Err)
	assert.True(t, j2.Executed)
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

	assert.Empty(t, p.JobsErrored)
	assert.Len(t, p.JobsExecuted, 1)

	assert.True(t, j.Executed)
	assert.NoError(t, j.Err)
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

	assert.Len(t, p.JobsErrored, 1)
	assert.Len(t, p.JobsExecuted, 1)
	assert.Equal(t, []string{"error"}, errorStrings(p.JobErrors()))

	assert.True(t, j.Executed)
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

	assert.Len(t, p.JobsErrored, 1)
	assert.Empty(t, p.JobsExecuted)

	err := p.JobErrors()[0]
	assert.Equal(t, "job panicked: error", err.Error())

	assert.False(t, j.Executed)
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

	assert.Len(t, p.JobsErrored, 1)
	assert.Empty(t, p.JobsExecuted)
	assert.Equal(t, []string{"job panicked: error"}, errorStrings(p.JobErrors()))

	assert.False(t, j.Executed)
	assert.Equal(t, "job panicked: error", j.Err.Error())
}

func errorStrings(errs []error) []string {
	strs := make([]string, len(errs))
	for i, err := range errs {
		strs[i] = err.Error()
	}
	return strs
}
