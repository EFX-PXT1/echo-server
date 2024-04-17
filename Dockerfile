#FROM gcr.io/iaas-gcr-reg-prd-ad3d/golden/scratch:1.0
# Build builder
FROM golang:1.22.2-alpine3.19 as build
# align to 1.21 for delve in 3.19
# FROM golang:1.21.9-alpine3.19 as build

ARG GOPROXY
ENV GOPROXY ${GOPROXY:-http://goproxy.gcp.ntg.equifax.com/}
ARG GOPRIVATE
ENV GOPRIVATE ${GOPRIVATE}
ARG GOSUMDB
ENV GOSUMDB ${GOSUMDB:-off}

#COPY _build/ /go/build/
COPY . /go/build/

# https://www.kenaqshal.com/blog/debugging-dockerized-go-applications
# CGO_ENABLED=0 go install -ldflags "-s -w -extldflags '-static'" github.com/go-delve/delve/cmd/dlv@v1.22.1 && \

RUN \
apk --no-cache upgrade && \
apk --no-cache add git make && \
apk --no-cache add --repository=https://dl-cdn.alpinelinux.org/alpine/edge/community delve && \
make -e -C /go/build

FROM gcr.io/iaas-gcr-reg-prd-ad3d/golden/os/alpine:3.19 as base
USER root
RUN \
deluser --remove-home efx_container_user

FROM base as debug

COPY --from=build --chmod=555 /usr/bin/dlv /usr/bin/dlv
COPY --from=build --chmod=555 /go/build/cmd/echo-server/echo-server-dbg /bin/echo-server-dbg

# On distroless 'nonroot' has uid `65532'
# On alpine     'nobody'  has uid `65534'
USER 65534:65534
ENV PORT 8080
EXPOSE 8080 4000
CMD [ "/usr/bin/dlv", "--check-go-version=false", "--listen=:4000", "--headless=true", "--log=true", "--accept-multiclient", "--api-version=2", "exec", "/bin/echo-server-dbg" ]

FROM base
# our single static binary
# COPY cmd/echo-server/echo-server /bin/echo-server
COPY --from=build --chmod=555 /go/build/cmd/echo-server/echo-server /bin/echo-server

# On distroless 'nonroot' has uid `65532'
# On alpine     'nobody'  has uid `65534'
USER 65534:65534
ENV PORT 8080
EXPOSE 8080
ENTRYPOINT ["/bin/echo-server"]
