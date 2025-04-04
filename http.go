package modulir

//go:generate go run scripts/embed_js/main.go

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/xerrors"
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

	if c.Websocket {
		mux.HandleFunc("/websocket.js", getWebsocketJSHandler(c))
		mux.HandleFunc("/websocket", getWebsocketHandler(c, buildComplete))
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%v", c.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // protect against Slowloris attack
	}

	go func() {
		err := server.ListenAndServe()

		// ListenAndServe always returns a non-nil error (but if started
		// successfully, it'll block for a long time).
		if !errors.Is(err, http.ErrServerClosed) {
			exitWithError(xerrors.Errorf("error starting HTTP server: %w", err))
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
	websocketPongWait = 10 * time.Second

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
	CheckOrigin: func(*http.Request) bool {
		// Thought about doing localhost only, but it may cause trouble for
		// something eventually. If end user can connect to the web page,
		// assume they're also safe for websockets.
		return true
	},
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
		c.Log.Infof(logPrefix(c, conn) + "Opened")
	}
}

func getWebsocketJSHandler(c *Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		err := websocketJSTemplate.Execute(w, map[string]interface{}{
			"Port": c.Port,
		})
		if err != nil {
			c.Log.Errorf("Error executing template/writing websocket.js: %v", err)
			return
		}
	}
}

// Produces a log prefix like `<WebSocket [::1]:53555>` which is colored if
// appropriate.
func logPrefix(c *Context, conn *websocket.Conn) string {
	return fmt.Sprintf(c.colorizer.Bold("<WebSocket %v> ").String(),
		conn.RemoteAddr())
}

func websocketReadPump(c *Context, conn *websocket.Conn, connClosed chan struct{}) {
	defer func() {
		conn.Close()
		connClosed <- struct{}{}
	}()

	conn.SetReadLimit(websocketMaxMessageSize)

	if err := conn.SetReadDeadline(time.Now().Add(websocketPongWait)); err != nil {
		c.Log.Errorf(logPrefix(c, conn)+"Couldn't set WebSocket read deadline: %v",
			err)
		return
	}

	conn.SetPongHandler(func(string) error {
		c.Log.Debugf(logPrefix(c, conn) + "Received pong")
		if err := conn.SetReadDeadline(time.Now().Add(websocketPongWait)); err != nil {
			c.Log.Errorf(logPrefix(c, conn)+"Couldn't set WebSocket read deadline: %v",
				err)
		}
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				c.Log.Infof(logPrefix(c, conn)+"Closed: %v", err)
			} else {
				c.Log.Errorf(logPrefix(c, conn)+"Error reading message: %v",
					err)
			}
			break
		}

		// We don't expect clients to send anything right now, so just ignore
		// incoming messages.
	}

	c.Log.Debugf(logPrefix(c, conn) + "Read pump ending")
}

func websocketWritePump(c *Context, conn *websocket.Conn,
	connClosed chan struct{}, buildComplete *sync.Cond,
) {
	ticker := time.NewTicker(websocketPingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	var done bool
	var writeErr error
	sendComplete := make(chan struct{}, 1)

	// This is a hack because of course there's no way to select on a
	// conditional variable. Instead, we have a separate Goroutine wait on the
	// conditional variable and signal the main select below through a channel.
	buildCompleteChan := make(chan struct{}, 1)
	go func() {
		for {
			buildComplete.L.Lock()
			buildComplete.Wait()
			buildComplete.L.Unlock()

			buildCompleteChan <- struct{}{}

			// Break out of the Goroutine when we can to prevent a Goroutine
			// leak.
			//
			// Unfortunately this isn't perfect. If we were sending a
			// build_complete, the Goroutine will die right away because the
			// wait below will fall through after the message was fully
			// received, and the client-side JavaScript will being the page
			// reload and close the websocket before that occurs. That's good.
			//
			// What isn't so good is that for other exit conditions like a
			// closed connection or a failed ping, the Goroutine will still be
			// waiting on the conditional variable's Wait above, and not exit
			// right away. The good news is that the next build event that
			// triggers will cause it to fall through and end the Goroutine. So
			// it will eventually be cleaned up, but that clean up may be
			// delayed.
			<-sendComplete
			if done {
				break
			}
		}

		c.Log.Debugf(logPrefix(c, conn) + "Build complete feeder ending")
	}()

	for {
		select {
		case <-buildCompleteChan:
			if err := conn.SetWriteDeadline(time.Now().Add(websocketWriteWait)); err != nil {
				c.Log.Errorf(logPrefix(c, conn)+"Couldn't set WebSocket read deadline: %v",
					err)
			}
			writeErr = conn.WriteJSON(websocketEvent{Type: "build_complete"})

			// Send shouldn't strictly need to be non-blocking, but we do one
			// anyway just to hedge against future or unexpected problems so as
			// not to accidentally stall out this loop.
			select {
			case sendComplete <- struct{}{}:
			default:
			}

		case <-connClosed:
			done = true

		case <-ticker.C:
			c.Log.Debugf(logPrefix(c, conn) + "Sending ping")
			if err := conn.SetWriteDeadline(time.Now().Add(websocketWriteWait)); err != nil {
				c.Log.Errorf(logPrefix(c, conn)+"Couldn't set WebSocket read deadline: %v",
					err)
			}
			writeErr = conn.WriteMessage(websocket.PingMessage, nil)
		}

		if writeErr != nil {
			c.Log.Errorf(logPrefix(c, conn)+"Error writing: %v",
				writeErr)
			done = true
		}

		if done {
			break
		}
	}

	c.Log.Debugf(logPrefix(c, conn) + "Write pump ending")
}
