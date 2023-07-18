FROM alpine:latest
RUN apk --update add ca-certificates
RUN mkdir -p /tmp

ARG USER_UID=10001
USER ${USER_UID}

COPY collector-console-config.yaml /etc/otel/config.yaml
COPY ../../collector-console/castai-collector-console /
EXPOSE 4317 55680 55679
ENTRYPOINT ["/castai-collector-console"]
CMD ["--config", "/etc/otel/config.yaml"]
