package modulir

import (
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Public
//
//
//
//////////////////////////////////////////////////////////////////////////////


// Listens for file system changes from fsnotify and pushes relevant ones back
// out over the rebuild channel.
//
// It doesn't start listening to fsnotify again until the main loop has
// signaled rebuildDone, so there is a possibility that in the case of very
// fast consecutive changes the build might not be perfectly up to date.
func watchChanges(c *Context, watchEvents chan fsnotify.Event, watchErrors chan error,
	rebuild chan map[string]struct{}, rebuildDone chan struct{}) {

	var changedSources, lastChangedSources map[string]struct{}
	var lastRebuild time.Time

	for {
		select {
		case event, ok := <-watchEvents:
			if !ok {
				c.Log.Infof("Watcher detected closed channel; stopping")
				return
			}

			c.Log.Debugf("Received event from watcher: %+v", event)
			lastChangedSources = changedSources
			changedSources = map[string]struct{}{event.Name: {}}

			if !shouldRebuild(event.Name, event.Op) {
				continue
			}

			// The central purpose of this loop is to make sure we do as few
			// build loops given incoming changes as possible.
			//
			// On the first receipt of a rebuild-eligible event we start
			// rebuilding immediately, and during the rebuild we accumulate any
			// other rebuild-eligible changes that stream in. When the initial
			// build finishes, we loop and start a new one if there were
			// changes since. If not, we return to the outer loop and continue
			// watching for fsnotify events.
			//
			// If changes did come in, the inner for loop continues to work --
			// triggering builds and accumulating changes while they're running
			// -- until we're able to successfully execute a build loop without
			// seeing a new change.
			//
			// The overwhelmingly common case will be few files being changed,
			// and therefore the inner for almost never needs to loop.
			for {
				if len(changedSources) < 1 {
					break
				}

				// If the detect changes are identical to the last set of
				// changes we just processed and we're within a certain quiesce
				// time, *don't* trigger another rebuild and just go back to
				// steady state.
				//
				// This is to protect against a problem where for a single save
				// operation, the watcher occasionally picks up a number of
				// events on the same file in quick succession, but not *so*
				// quick that the build can't finish before the next one comes
				// in. The faster the build, the more often this is a problem.
				//
				// I'm not sure why this occurs, but protect against it.
				if buildWithinSameFileQuiesce(lastRebuild, time.Now(), changedSources, lastChangedSources) {
					c.Log.Infof("Identical file(s) %v changed within quiesce time; not rebuilding",
						mapKeys(changedSources))
					break
				}

				lastRebuild = time.Now()

				// Start rebuild
				rebuild <- changedSources

				// Zero out the set of changes and start accumulating.
				//
				// Keep a pointer to it so that we can compare it to any new
				// set of changes.
				lastChangedSources = changedSources
				changedSources = nil

				// Wait until rebuild is finished. In the meantime, accumulate
				// new events that come in on the watcher's channel and prepare
				// for the next loop.
			INNER_LOOP:
				for {
					select {
					case <-rebuildDone:
						// Break and start next outer loop
						break INNER_LOOP

					case event, ok := <-watchEvents:
						if !ok {
							c.Log.Infof("Watcher detected closed channel; stopping")
							return
						}

						if !shouldRebuild(event.Name, event.Op) {
							continue
						}

						if changedSources == nil {
							changedSources = make(map[string]struct{})
						}

						changedSources[event.Name] = struct{}{}

					case err, ok := <-watchErrors:
						if !ok {
							c.Log.Infof("Watcher detected closed channel; stopping")
							return
						}
						c.Log.Errorf("Error from watcher:", err)
					}
				}
			}

		case err, ok := <-watchErrors:
			if !ok {
				c.Log.Infof("Watcher detected closed channel; stopping")
				return
			}
			c.Log.Errorf("Error from watcher:", err)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Private
//
//
//
//////////////////////////////////////////////////////////////////////////////

// The time window in which *not* to trigger a rebuild if the next set of
// detected changes are on exactly the same files as the last.
const sameFileQuiesceTime = 100 * time.Millisecond

// See comment over this function's invocation.
func buildWithinSameFileQuiesce(lastRebuild, now time.Time,
	changedSources, lastChangedSources map[string]struct{}) bool {

	if lastChangedSources == nil {
		return false
	}

	if now.Add(-sameFileQuiesceTime).After(lastRebuild) {
		return false
	}

	// Acts as a quick compare so avoid using DeepEqual if possible
	if len(lastChangedSources) != len(changedSources) {
		return false
	}

	return reflect.DeepEqual(lastChangedSources, changedSources)
}

// Decides whether a rebuild should be triggered given some input event
// properties from fsnotify.
func shouldRebuild(path string, op fsnotify.Op) bool {
	base := filepath.Base(path)

	// Mac OS' worst mistake.
	if base == ".DS_Store" {
		return false
	}

	// Vim creates this temporary file to see whether it can write into a
	// target directory. It screws up our watching algorithm, so ignore it.
	if base == "4913" {
		return false
	}

	// A special case, but ignore creates on files that look like Vim backups.
	if strings.HasSuffix(base, "~") {
		return false
	}

	if op&fsnotify.Create != 0 {
		return true
	}

	if op&fsnotify.Remove != 0 {
		return true
	}

	if op&fsnotify.Write != 0 {
		return true
	}

	// Ignore everything else. Rationale:
	//
	//   * chmod: We don't really care about these as they won't affect build
	//     output. (Unless potentially we no longer can read the file, but
	//     we'll go down that path if it ever becomes a problem.)
	//
	//   * rename: Will produce a following create event as well, so just
	//     listen for that instead.
	//
	return false
}
