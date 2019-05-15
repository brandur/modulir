package modulir

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"github.com/fsnotify/fsnotify"
)

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
	finish := make(chan struct{}, 1)
	rebuild := make(chan map[string]struct{}, 1)
	rebuildDone := make(chan struct{}, 1)

	go watchChanges(newContext(), watchEvents, watchErrors,
		finish, rebuild, rebuildDone)

	// Finish up
	finish <- struct{}{}
}

// Helper to easily create a new Modulir context.
func newContext() *Context {
	return NewContext(&Args{Log: &Logger{Level: LevelInfo}})
}
