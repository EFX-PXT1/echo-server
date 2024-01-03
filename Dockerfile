FROM gcr.io/iaas-gcr-reg-prd-ad3d/golden/os/alpine:3.19
USER root
# remove openssl and libs, results in following purge
# (1/7) Purging apk-tools (2.14.0-r5)
# (2/7) Purging ca-certificates-bundle (20230506-r0)
# (3/7) Purging ca-certificates (20230506-r0)
# Executing ca-certificates-20230506-r0.post-deinstall
# (4/7) Purging ssl_client (1.36.1-r17)
# (5/7) Purging zlib (1.3-r2)
# (6/7) Purging libssl3 (3.1.4-r2)
# (7/7) Purging libcrypto3 (3.1.4-r2)
RUN apk --no-cache del -r openssl libcrypto3 libssl3 && \
  rm -rf /var/cache/apk/*
# our single static binary
COPY cmd/echo-server/echo-server /bin/echo-server
# On distroless 'nonroot' has uid `65532'
# On alpine     'nobody'  has uid `65534'
USER 65534:65534
ENV PORT 8080
EXPOSE 8080
ENTRYPOINT ["/bin/echo-server"]
