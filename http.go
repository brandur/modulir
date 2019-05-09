package modulir

import (
	"fmt"
	"net"
	"net/http"
	"path"

	"github.com/brandur/modulir/context"
	"github.com/rcrowley/goagain"
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

// getGoagainListener facilitates graceful restarts by trying to inherit a
// listener from a parent process that's forked it. If there wasn't one, it
// opens a new listener instead.
func getGoagainListener(c *context.Context) (net.Listener, error) {
	listener, listenErr := goagain.Listener()

	if listenErr != nil {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%v", c.Port))
		if err != nil {
			return nil, err
		}
		c.Log.Infof("Started a fresh listener on port %v", c.Port)
		return listener, nil
	}

	// We've inherited the listener and are about ready to serve content, so
	// kill our child process.
	if err := goagain.Kill(); err != nil {
		return nil, err
	}
	
	c.Log.Infof("Inherited a listener on port %v", c.Port)
	return listener, nil
}

func serveTargetDirHTTP(c *context.Context, listener net.Listener) error {
	c.Log.Infof("Serving '%s' from: http://localhost:%v/", path.Clean(c.TargetDir), c.Port)
	handler := http.FileServer(http.Dir(c.TargetDir))
	return http.Serve(listener, handler)
}

