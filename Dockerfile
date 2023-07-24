FROM alpine:latest
RUN apk --update add ca-certificates
COPY collector-config.yaml /etc/otel/config.yaml
COPY castai-collector/castai-collector /
ENTRYPOINT ["/castai-collector"]
CMD ["--config", "/etc/otel/config.yaml"]