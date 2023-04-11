# Echo Server

A very simple HTTP echo server with support for websockets.

## Behavior
- Any messages sent from a websocket client are echoed
- Visit `/.ws` for a basic UI to connect and send websocket messages
- Requests to any other URL will return the request headers and body

## Configuration

Environment variables are used to set the behaviour.

- `PORT` sets the server port, which defaults to `8080`
- Define `LOG_HTTP_BODY` to dump request bodies to `STDOUT`
- Define `LOG_ALL` to log a line to `STDOUT` for each request
- `ENABLE_FEATURES` is a comma or space separated list of features to enable
- Define `META_FILE` as the filename of a colon separated key:value metadata

### Features

Additional functionality can be requested by the addition of query parameters.
By default this functionality is not enabled, but can be adding to the list of
enabled features.

- `delay` to allow requests to be delayed to simulate load (period with units)
- `headers` to additionally output the request headers
- `env` to additionally output the process environment
- `meta` to additionally output the metadata
- `log` to log a request line when `LOG_ALL` is not set

## Running the server

The examples below show a few different ways of running the server with the HTTP
server bound to a custom TCP port of `10000`.

### Running locally

```
GO111MODULE=off go get -u github.com/jmalloc/echo-server/...
PORT=10000 echo-server
```

### Running under Docker

To run as a container:

```
docker run --detach -p 10000:8080 jmalloc/echo-server
```

To run as a service:

```
docker service create --publish 10000:8080 jmalloc/echo-server
```
