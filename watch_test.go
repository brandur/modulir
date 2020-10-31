package modulir

import (
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	assert "github.com/stretchr/testify/require"
)

func TestBuildWithinSameFileQuiesce(t *testing.T) {
	baseTime := time.Now()

	lastChanges := map[string]struct{}{"a/path": {}}
	sameLastChanges := map[string]struct{}{"a/path": {}}
	diffLastChanges := map[string]struct{}{"b/path": {}}

	// No last changes
	assert.False(t, buildWithinSameFileQuiesce(
		baseTime, baseTime, lastChanges, nil,
	))

	// Rebuild after quiesce time
	assert.False(t, buildWithinSameFileQuiesce(
		baseTime, baseTime.Add(10*time.Second), lastChanges, sameLastChanges,
	))

	// Different set of canges
	assert.False(t, buildWithinSameFileQuiesce(
		baseTime, baseTime, lastChanges, diffLastChanges,
	))

	// Within quiesce time and same set of changes
	assert.True(t, buildWithinSameFileQuiesce(
		baseTime, baseTime, lastChanges, sameLastChanges,
	))
}

func TestShouldRebuild(t *testing.T) {
	// Most things signal a rebuild
	assert.Equal(t, true, shouldRebuild("a/path", fsnotify.Create))
	assert.Equal(t, true, shouldRebuild("a/path", fsnotify.Remove))
	assert.Equal(t, true, shouldRebuild("a/path", fsnotify.Write))

	// With just a few special cases that don't
	assert.Equal(t, false, shouldRebuild("a/path", fsnotify.Chmod))
	assert.Equal(t, false, shouldRebuild("a/path", fsnotify.Rename))
	assert.Equal(t, false, shouldRebuild("a/.DS_Store", fsnotify.Create))
	assert.Equal(t, false, shouldRebuild("a/4913", fsnotify.Create))
	assert.Equal(t, false, shouldRebuild("a/path~", fsnotify.Create))
}

func TestWatchChanges(t *testing.T) {
	watchEvents := make(chan fsnotify.Event, 1)
	watchErrors := make(chan error, 1)
	rebuild := make(chan map[string]struct{}, 1)
	rebuildDone := make(chan struct{}, 1)

	go watchChanges(newContext(), watchEvents, watchErrors,
		rebuild, rebuildDone)

	{
		// An ineligible even that will be ignored.
		watchEvents <- fsnotify.Event{Name: "a/path~", Op: fsnotify.Create}

		select {
		case <-rebuild:
			assert.Fail(t, "Should not have received rebuild on ineligible event")
		case <-time.After(50 * time.Millisecond):
		}
	}

	{
		// An valid event.
		watchEvents <- fsnotify.Event{Name: "a/path", Op: fsnotify.Create}

		select {
		case sources := <-rebuild:
			assert.Equal(t, map[string]struct{}{"a/path": {}}, sources)
		case <-time.After(50 * time.Millisecond):
			assert.Fail(t, "Should have received a rebuild signal")
		}

		// While we're rebuilding, the watcher will accumulate events. Send a
		// few more that are eligible and one that's not.
		watchEvents <- fsnotify.Event{Name: "a/path1", Op: fsnotify.Create}
		watchEvents <- fsnotify.Event{Name: "a/path2", Op: fsnotify.Create}
		watchEvents <- fsnotify.Event{Name: "a/path~", Op: fsnotify.Create}

		// Signal that the build is finished
		rebuildDone <- struct{}{}

		// Now verify that we got the accumulated changes.
		select {
		case sources := <-rebuild:
			assert.Equal(t, map[string]struct{}{
				"a/path1": {},
				"a/path2": {},
			}, sources)
		case <-time.After(50 * time.Millisecond):
			assert.Fail(t, "Should have received a rebuild signal")
		}

		// Send one more rebuild done so the watcher can continue
		rebuildDone <- struct{}{}
	}

	// Finish up by closing the channel to stop the loop
	close(watchEvents)
}

// Helper to easily create a new Modulir context.
func newContext() *Context {
	return NewContext(&Args{Log: &Logger{Level: LevelInfo}})
}
