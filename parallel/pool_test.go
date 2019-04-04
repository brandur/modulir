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

	p.JobsChan <- func() error { return nil }
	p.JobsChan <- func() error { return nil }
	p.JobsChan <- func() error { return nil }
	p.Wait()

	assert.Equal(t, []error(nil), p.Errors)
}

func TestWithError(t *testing.T) {
	p := NewPool(&log.Logger{Level: log.LevelDebug}, 10)
	p.Run()

	p.JobsChan <- func() error { return nil }
	p.JobsChan <- func() error { return nil }
	p.JobsChan <- func() error { return fmt.Errorf("error") }
	p.Wait()

	assert.Equal(t, []error{fmt.Errorf("error")}, p.Errors)
}
