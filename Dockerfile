#FROM gcr.io/iaas-gcr-reg-prd-ad3d/golden/scratch:1.0
# Build builder
FROM golang:1.22.2-alpine3.19 as build

ARG GOPROXY
ENV GOPROXY ${GOPROXY:-http://goproxy.gcp.ntg.equifax.com/}
ARG GOPRIVATE
ENV GOPRIVATE ${GOPRIVATE}
ARG GOSUMDB
ENV GOSUMDB ${GOSUMDB:-off}

#COPY _build/ /go/build/
COPY . /go/build/

RUN \
apk --no-cache upgrade && \
apk --no-cache add git make && \
make -e -C /go/build

FROM gcr.io/iaas-gcr-reg-prd-ad3d/golden/os/alpine:3.19
USER root
RUN \
deluser --remove-home efx_container_user

# our single static binary
# COPY cmd/echo-server/echo-server /bin/echo-server
COPY --from=build --chmod=555 /go/build/cmd/echo-server/echo-server /bin/echo-server

# On distroless 'nonroot' has uid `65532'
# On alpine     'nobody'  has uid `65534'
USER 65534:65534
ENV PORT 8080
EXPOSE 8080
ENTRYPOINT ["/bin/echo-server"]
