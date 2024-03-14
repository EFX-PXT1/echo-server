FROM gcr.io/iaas-gcr-reg-prd-ad3d/golden/scratch:1.0
# our single static binary
COPY cmd/echo-server/echo-server /bin/echo-server
# On distroless 'nonroot' has uid `65532'
# On alpine     'nobody'  has uid `65534'
USER 65534:65534
ENV PORT 8080
EXPOSE 8080
ENTRYPOINT ["/bin/echo-server"]
