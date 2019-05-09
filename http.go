package modulir

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/brandur/modulir/context"
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

func serveHTTP(c *context.Context) {
	c.Log.Infof("Serving '%s' on port %v", path.Clean(c.TargetDir), c.Port)
	c.Log.Infof("Open browser to: http://localhost:%v/", c.Port)
	handler := http.FileServer(http.Dir(c.TargetDir))
	err := http.ListenAndServe(fmt.Sprintf(":%v", c.Port), handler)
	if err != nil {
		c.Log.Errorf("Error starting server: %v", err)
		os.Exit(1)
	}
}

