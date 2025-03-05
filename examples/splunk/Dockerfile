FROM alpine:latest
RUN apk --update add ca-certificates

ARG TARGETARCH
COPY castai-collector/castai-collector-${TARGETARCH} /castai-collector
COPY examples/splunk/collector-config.yaml /etc/otel/config.yaml

ENTRYPOINT ["/castai-collector"]
CMD ["--config", "/etc/otel/config.yaml"]