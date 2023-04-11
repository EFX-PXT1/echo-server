package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var meta string
var feature map[string]bool

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Echo server listening on port %s.\n", port)

	// populate meta data
	if metaFile, ok := os.LookupEnv("META_FILE"); ok {
		if data, err := os.ReadFile(metaFile); err == nil {
			meta = string(data[:])
		}
	}

	// setup features
	setupFeatures()

	ctx := signalContext()

	err := http.ListenAndServe(
		":"+port,
		h2c.NewHandler(
			http.HandlerFunc(handler),
			&http2.Server{},
		),
	)
	if err != nil {
		panic(err)
	}
	<-ctx.Done()
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

func setupFeatures() {
	// populates features map from environment variable
	features := os.Getenv("ENABLE_FEATURES")
	feature = make(map[string]bool)
	feature["delay"] = ContainsI(features, "delay")
	feature["headers"] = ContainsI(features, "headers")
	feature["env"] = ContainsI(features, "env")
	feature["meta"] = ContainsI(features, "meta")
	feature["log"] = ContainsI(features, "log")
}

func ContainsI(a string, b string) bool {
	return strings.Contains(
		strings.ToLower(a),
		strings.ToLower(b),
	)
}

func handler(wr http.ResponseWriter, req *http.Request) {
	_, log := req.URL.Query()["log"]
	if os.Getenv("LOG_HTTP_BODY") != "" {
		fmt.Printf("--------  %s | %s %s\n", req.RemoteAddr, req.Method, req.URL)
		buf := &bytes.Buffer{}
		buf.ReadFrom(req.Body)

		if buf.Len() != 0 {
			w := hex.Dumper(os.Stdout)
			w.Write(buf.Bytes())
			w.Close()
		}

		// Replace original body with buffered version so it's still sent to the
		// browser.
		req.Body.Close()
		req.Body = ioutil.NopCloser(
			bytes.NewReader(buf.Bytes()),
		)
	} else if os.Getenv("LOG_ALL") != "" || (log && feature["log"]) {
		fmt.Printf("%s | %s %s\n", req.RemoteAddr, req.Method, req.URL)
	}

	if websocket.IsWebSocketUpgrade(req) {
		serveWebSocket(wr, req)
	} else if req.URL.Path == "/.ws" {
		wr.Header().Add("Content-Type", "text/html")
		wr.WriteHeader(200)
		io.WriteString(wr, websocketHTML)
	} else {
		serveHTTP(wr, req)
	}
}

func serveWebSocket(wr http.ResponseWriter, req *http.Request) {
	connection, err := upgrader.Upgrade(wr, req, nil)
	if err != nil {
		fmt.Printf("%s | %s\n", req.RemoteAddr, err)
		return
	}

	defer connection.Close()
	fmt.Printf("%s | upgraded to websocket\n", req.RemoteAddr)

	var message []byte

	host, err := os.Hostname()
	if err == nil {
		message = []byte(fmt.Sprintf("Request served by %s", host))
	} else {
		message = []byte(fmt.Sprintf("Server hostname unknown: %s", err.Error()))
	}

	err = connection.WriteMessage(websocket.TextMessage, message)
	if err == nil {
		var messageType int

		for {
			messageType, message, err = connection.ReadMessage()
			if err != nil {
				break
			}

			if messageType == websocket.TextMessage {
				fmt.Printf("%s | txt | %s\n", req.RemoteAddr, message)
			} else {
				fmt.Printf("%s | bin | %d byte(s)\n", req.RemoteAddr, len(message))
			}

			err = connection.WriteMessage(messageType, message)
			if err != nil {
				break
			}
		}
	}

	if err != nil {
		fmt.Printf("%s | %s\n", req.RemoteAddr, err)
	}
}

func serveHTTP(wr http.ResponseWriter, req *http.Request) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)

	host, err := os.Hostname()
	if err == nil {
		fmt.Fprintf(wr, "Request served by %s\n\n", host)
	} else {
		fmt.Fprintf(wr, "Server hostname unknown: %s\n\n", err.Error())
	}

	fmt.Fprintf(wr, "%s %s %s\n", req.Proto, req.Method, req.URL)
	fmt.Fprintln(wr, "")

	// delay response if requested
	delays := req.URL.Query()["delay"]
	if len(delays) > 0 && feature["delay"] {
		if d, err := time.ParseDuration(delays[0]); err == nil {
			time.Sleep(d)
			fmt.Fprintf(wr, "Delayed by: %s\n\n", delays[0])
		}
	}

	// output request headers if requested
	if _, ok := req.URL.Query()["headers"]; ok && feature["headers"] {
		fmt.Fprintf(wr, "Host: %s\n", req.Host)
		for key, values := range req.Header {
			for _, value := range values {
				fmt.Fprintf(wr, "%s: %s\n", key, value)
			}
		}
		fmt.Fprintln(wr, "")
	}

	// dump environment if requested
	if _, ok := req.URL.Query()["env"]; ok && feature["env"] {
		for _, e := range os.Environ() {
			fmt.Fprintf(wr, "%s\n", e)
			//			pair := strings.SplitAfterN(e, "=", 2)
			//			fmt.Fprintf(wr, "%s: %s\n", pair[0], pair[1])
		}
		fmt.Fprintln(wr, "")
	}

	// dump meta if requested
	if _, ok := req.URL.Query()["meta"]; ok && feature["meta"] {
		fmt.Fprintf(wr, "%s\n\n", meta)
	}

	io.Copy(wr, req.Body)
}
