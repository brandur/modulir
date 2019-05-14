package modulir

//go:generate go run scripts/embed_js/main.go

import (
	"fmt"
	"net/http"
	"path"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
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

// Starts serving the built site over HTTP on the configured port. A server
// instance is returned so that it can be shut down gracefully.
func startServingTargetDirHTTP(c *Context, buildComplete *sync.Cond) *http.Server {
	c.Log.Infof("Serving '%s' to: http://localhost:%v/", path.Clean(c.TargetDir), c.Port)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(c.TargetDir)))

	if c.StartWebsocket {
		mux.HandleFunc("/websocket.js", getWebsocketJSHandler(c))
		mux.HandleFunc("/websocket", getWebsocketHandler(c, buildComplete))
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", c.Port),
		Handler: mux,
	}

	go func() {
		err := server.ListenAndServe()

		// ListenAndServe always returns a non-nil error (but if started
		// successfully, it'll block for a long time).
		if err != http.ErrServerClosed {
			exitWithError(errors.Wrap(err, "Error starting HTTP server"))
		}
	}()

	return server
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

// A type representing the extremely basic messages that we'll be serializing
// and sending back over a websocket.
type websocketEvent struct {
	Type string `json:"type"`
}

const (
	// Maximum message size allowed from peer.
	websocketMaxMessageSize = 512

	// The frequency at which to send pings back to clients connected over a
	// websocket. Must be less than websocketPongWait.
	websocketPingPeriod = (websocketPongWait * 9) / 10

	// Time allowed to read the next pong message from the peer.
	websocketPongWait = 30 * time.Second

	// Time allowed to write a message to the peer.
	websocketWriteWait = 10 * time.Second
)

// A template that will render the websocket JavaScript code that connecting
// clients will load and run. The `websocketJS` source of this template comes
// from `js.go` which is generated from sources found in the `./js` directory
// with `go generate`.
var websocketJSTemplate = template.Must(template.New("websocket.js").Parse(websocketJS))

// Part of the Gorilla websocket infrastructure that upgrades HTTP connections
// to websocket connections when we see an incoming websocket request.
var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func getWebsocketHandler(c *Context, buildComplete *sync.Cond) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocketUpgrader.Upgrade(w, r, nil)
		if err != nil {
			c.Log.Errorf("Error upgrading websocket connection: %v", err)
			return
		}

		connClosed := make(chan struct{}, 1)

		go websocketReadPump(c, conn, connClosed)
		go websocketWritePump(c, conn, connClosed, buildComplete)

		c.Log.Infof("Websocket opened")
	}
}

func getWebsocketJSHandler(c *Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		err := websocketJSTemplate.Execute(w, map[string]interface{}{
			"Port": c.Port,
		});

		if err != nil {
			c.Log.Errorf("Error executing template/writing websocket.js: %v", err)
			return
		}
	}
}

func websocketReadPump(c *Context, conn *websocket.Conn, connClosed chan struct{}) {
	defer func() {
		conn.Close()
		connClosed <- struct{}{}
	}()

	conn.SetReadLimit(websocketMaxMessageSize)

	conn.SetReadDeadline(time.Now().Add(websocketPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(websocketPongWait))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				c.Log.Infof("Websocket closed: %v", err)
			}
			break
		}

		// We don't expect clients to send anything right now, so just ignore
		// incoming messages.
	}
}

func websocketWritePump(c *Context, conn *websocket.Conn,
	connClosed chan struct{}, buildComplete *sync.Cond) {

	ticker := time.NewTicker(websocketPingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	// This is a hack because of course there's no way to select on a
	// conditional variable.
	buildCompleteChan := make(chan struct{})
	go func() {
		for {
			buildComplete.L.Lock()
			buildComplete.Wait()
			buildCompleteChan <- struct{}{}
			buildComplete.L.Unlock()
		}
	}()

	var err error

	for {
		select {
		case <-buildCompleteChan:
			conn.SetWriteDeadline(time.Now().Add(websocketWriteWait))
			if err = conn.WriteJSON(websocketEvent{Type: "build_complete"}); err != nil {
				goto errored
			}

		case <-connClosed:
			goto finished

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(websocketWriteWait))
			if err = conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				goto errored
			}
		}
	}

errored:
	if err != nil {
		c.Log.Errorf("Error writing to websocket: %v", err)
	}

finished:
}