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
	"log"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/filters"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
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

	logInit()

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
			slog.Error("setupOTelSDK", "error", err)
			return
		}
		// Handle shutdown properly so nothing leaks.
		defer func() {
			err = errors.Join(err, otelShutdown(context.Background()))
		}()

		var opts []otelhttp.Option
		if !feature["traceoptions"] {
			opts = append(opts, otelhttp.WithFilter(filters.Not(filters.Method("OPTIONS"))))
		}
		handl = otelhttp.NewHandler(handl, "", opts...)
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
	feature["timeout"] = ContainsI(features, "timeout")
	feature["traceoptions"] = ContainsI(features, "traceoptions")
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
		req.Body = io.NopCloser(
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
	} else if req.Method == "OPTIONS" {
		if feature["traceoptions"] {
			serveGET(wr, req, true)
		} else {
			serveGET(wr, req, false)
		}
	} else {
		serveGET(wr, req, true)
	}
}

type Chain struct {
	URL []string `json:"chain"`
}

func servePOST(wr http.ResponseWriter, req *http.Request) {
	// Example
	// curl -XPOST http://localhost:8080/?headers -d '{ "chain": ["http://localhost:8080/?headers&think=300ms&delay=300ms", "http://localhost:8080/?headers&think=150ms&delay=2000ms"]}'
	// replace & with ^& on windos

	// inspect and parse body for a chain
	reqBody, _ := io.ReadAll(req.Body)
	chain := Chain{}
	err := json.Unmarshal([]byte(reqBody), &chain)
	if err != nil {
		log.Printf("error: %v", err)
	}

	if len(chain.URL) == 0 {
		// no chain so act like get
		serveGET(wr, req, true)
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

	// default timeout for post request
	timeout := 60 * time.Second

	// override timeout if allowed and provided
	timeoutS := req.URL.Query()["timeout"]
	if len(timeoutS) > 0 && feature["timeout"] {
		if d, err := time.ParseDuration(timeoutS[0]); err == nil {
			timeout = d
		}
	}

	// think delay before chain link if requested
	delays := req.URL.Query()["think"]
	if len(delays) > 0 && feature["think"] {
		if d, err := time.ParseDuration(delays[0]); err == nil {
			time.Sleep(d)
			fmt.Fprintf(wr, "Thinking for: %s\n\n", delays[0])
		}
	}

	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/

	ctx := req.Context()
	// tr := otel.Tracer("echo-server/client")
	tr := trace.SpanFromContext(ctx).TracerProvider().Tracer("echo-server/client")

	httptr := &http.Transport{
		// disable tls checks on http post calls
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	//	client := &http.Client{Transport: htr}
	client := &http.Client{Transport: otelhttp.NewTransport(
		httptr,
		otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
			return otelhttptrace.NewClientTrace(ctx)
		})),
		Timeout: timeout,
	}

	body, err := func(ctx context.Context) (body []byte, err error) {
		ctx, span := tr.Start(ctx, "invoke_chain", trace.WithAttributes(semconv.ProcessCommand("echo-server")))
		defer span.End()
		postReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(chainBody))

		// propagate tracing headers
		// https://istio.io/latest/about/faq/distributed-tracing/#how-to-support-tracing

		// create new outgoing trace and inject into outgoing request
		//		ctx, postReq = otelhttptrace.W3C(ctx, postReq)
		//		otelhttptrace.Inject(ctx, postReq)

		_, postReq = PropagateEfxHeaders(ctx, req, postReq)

		// call next link in chain
		// resp, err := client.Post(url, "application/json", bytes.NewReader(chainBody))
		postReq.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(postReq)
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close() // think otel requires close
		}
		return
	}(ctx)

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

	wr.Write(body)
}

func serveGET(wr http.ResponseWriter, req *http.Request, startSpan bool) {
	if startSpan {
		ctx := req.Context()
		tr := otel.Tracer("echo-server/server")
		_, span := tr.Start(ctx, "serveGET", trace.WithAttributes(semconv.ProcessCommand("echo-server")))
		defer span.End()
	}

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

func PropagateEfxHeaders(ctx context.Context, src *http.Request, req *http.Request) (context.Context, *http.Request) {
	// https://istio.io/latest/docs/tasks/observability/distributed-tracing/overview/#trace-context-propagation
	efxHeaders := []string{"x-request-id"}

	// for now we just copy from src to req, but in future we may have hierarchy and also look in ctx
	for _, key := range efxHeaders {
		if id := src.Header.Get(key); id != "" {
			if req.Header.Get(key) == "" {
				req.Header.Add(key, id)
			}
		}
	}
	return ctx, req
}
