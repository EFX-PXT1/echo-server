package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

	ctx := context.Background()
	if !feature["nosignals"] {
		ctx = signalContext()
	}

	// setup handler
	handl := h2c.NewHandler(
		http.HandlerFunc(handler),
		&http2.Server{},
	)

	if feature["otel"] {
		// Set up OpenTelemetry.
		serviceName := "echo-server"
		serviceVersion := "0.0.1"
		otelShutdown, err := setupOTelSDK(ctx, serviceName, serviceVersion)
		if err != nil {
			return
		}
		// Handle shutdown properly so nothing leaks.
		defer func() {
			err = errors.Join(err, otelShutdown(context.Background()))
		}()

		handl = otelhttp.NewHandler(handl, "")
	}

	err := http.ListenAndServe(":"+port, handl)
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
	feature["nosignals"] = ContainsI(features, "nosignals")
	feature["delay"] = ContainsI(features, "delay")
	feature["think"] = ContainsI(features, "think")
	feature["headers"] = ContainsI(features, "headers")
	feature["env"] = ContainsI(features, "env")
	feature["meta"] = ContainsI(features, "meta")
	feature["log"] = ContainsI(features, "log")
	feature["otel"] = ContainsI(features, "otel")
	feature["post"] = ContainsI(features, "post")
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
	if req.Method == "POST" && feature["post"] {
		servePOST(wr, req)
	} else {
		serveGET(wr, req)
	}
}

type Chain struct {
	URL []string `json:"chain"`
}

func servePOST(wr http.ResponseWriter, req *http.Request) {
	// Example
	// curl -XPOST http://localhost:8080/?headers -d '{ "chain": ["http://localhost:8080/?headers&think=300ms&delay=300ms", "http://localhost:8080/?headers&think=150ms&delay=2000ms"]}'

	// inspect and parse body for a chain
	reqBody, _ := io.ReadAll(req.Body)
	chain := Chain{}
	err := json.Unmarshal([]byte(reqBody), &chain)
	if err != nil {
		log.Printf("error: %v", err)
	}

	if len(chain.URL) == 0 {
		// no chain so act like get
		serveGET(wr, req)
		return
	}

	// pop front of chain and prepare new chainBody
	url := chain.URL[0]
	chain.URL = chain.URL[1:]
	chainBody, _ := json.Marshal(chain)

	// output req
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)

	host, err := os.Hostname()
	if err == nil {
		fmt.Fprintf(wr, "Request received by %s at %s\n\n", host, time.Now().Format(time.RFC3339Nano))
	} else {
		fmt.Fprintf(wr, "Server hostname unknown: %s\n\n", err.Error())
	}

	fmt.Fprintf(wr, "%s %s %s\n", req.Proto, req.Method, req.URL)
	fmt.Fprintln(wr, "")

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

	fmt.Fprintf(wr, "%s\n", reqBody)
	fmt.Fprintln(wr, "")

	// think delay before chain link if requested
	delays := req.URL.Query()["think"]
	if len(delays) > 0 && feature["think"] {
		if d, err := time.ParseDuration(delays[0]); err == nil {
			time.Sleep(d)
			fmt.Fprintf(wr, "Thinking for: %s\n\n", delays[0])
		}
	}

	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/

	// disable tls checks on http post calls
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	postReq, _ := http.NewRequest("POST", url, bytes.NewReader(chainBody))

	// propagate tracing headers
	// https://istio.io/latest/about/faq/distributed-tracing/#how-to-support-tracing
	postReq.Header = http.Header{
		"x-request-id":                req.Header.Values("x-request-id"),
		"x-b3-traceid":                req.Header.Values("x-b3-traceid"),
		"x-b3-spanid":                 req.Header.Values("x-b3-spanid"),
		"x-b3-parentspanid":           req.Header.Values("x-b3-parentspanid"),
		"x-b3-sampled":                req.Header.Values("x-b3-sampled"),
		"x-b3-flags":                  req.Header.Values("x-b3-flags"),
		"x-ot-span-context":           req.Header.Values("x-ot-span-context"),
		"x-datadog-trace-id":          req.Header.Values("x-datadog-trace-id"),
		"x-datadog-parent-id":         req.Header.Values("x-datadog-parent-id"),
		"x-datadog-sampling-priority": req.Header.Values("x-datadog-sampling-priority"),
	}

	// call next link in chain
	// resp, err := client.Post(url, "application/json", bytes.NewReader(chainBody))
	resp, err := client.Do(postReq)

	if err != nil {
		fmt.Fprintf(wr, "chain call failed with %s\n", err)
		fmt.Fprintln(wr, "")
		return
	}

	// delay response if requested
	delays = req.URL.Query()["delay"]
	if len(delays) > 0 && feature["delay"] {
		if d, err := time.ParseDuration(delays[0]); err == nil {
			time.Sleep(d)
			fmt.Fprintf(wr, "Delayed by: %s\n\n", delays[0])
		}
	}

	io.Copy(wr, resp.Body)
}

func serveGET(wr http.ResponseWriter, req *http.Request) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)

	host, err := os.Hostname()
	if err == nil {
		fmt.Fprintf(wr, "Request received by %s at %s\n\n", host, time.Now().Format(time.RFC3339Nano))
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
