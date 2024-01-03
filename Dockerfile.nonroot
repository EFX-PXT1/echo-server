FROM gcr.io/distroless/static:nonroot
COPY cmd/echo-server/echo-server /bin/echo-server
USER nonroot:nonroot
ENV PORT 8080
EXPOSE 8080
ENTRYPOINT ["/bin/echo-server"]
