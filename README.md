<a href="https://cast.ai">
    <img src="https://cast.ai/wp-content/themes/cast/img/cast-logo-dark-blue.svg" align="right" height="100" />
</a>

CAST AI Audit Logs Collector Receiver
==================

This repository contains Audit Logs Receiver that can be used for building custom Open Telemetry Collector.
Additional tools / instrumentation / examples are provided for smooth experience of setting things up:
- Building and compiling Open Telemetry Collector using Make files
- Building and hosting Docker image
- Helm chart for running collector on k8s
- Several Open Telemetry examples with different destinations (file, Grafana Loki, Coralogix)


### Setting things up

CAST AI Audit Logs receiver is not part of ['standard' receivers provided by Open Telemetry hosted here](https://github.com/open-telemetry/opentelemetry-collector-contrib).
So it requires building a custom Open Telemetry Collector (a program that combines selected receivers, processors and exporters into a pipeline used for pushing logs / metrics / traces).

The first step in building a custom Collector is installing required tools, which can be done as simple as running:
```
make setup
```

It installs:
- [Open Telemetry Metadata Generator](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/mdatagen) -
it is used to generate receiver's definition (metadata about receiver itself); for example, stability level, is this a logs or metrics receiver, etc. Audit Logs Exporter's [metadata is defined here](./auditlogsreceiver/metadata.yaml).
- [Open Telemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) - 
builder is required to generate a code that bootstraps selected components so compilation may produce an executable binary. Builder's [configuration is defined here](./builder-config.yaml)

Collector can be customized (what gets included in binary artifact) as needed by tailoring `builder-config.yaml` if needed.
Refer to OpenTelemetry Collector Contrib Distro's (for example, [the manifest](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/main/distributions/otelcol-contrib/manifest.yaml) for a full list of available components.

### Building and running a custom Controller

Building a custom Collector is as simple as:
```
make build
```

It produces few artifacts (including a binary executable file) into `castai-collector` directory.
To run the newly built binary, use:
```
./castai-collector/castai-collector --config collector-config.yaml
```

or instead it can be run as make target:
```
make run
```

### Build and run the Docker Image
Build the Docker image with:
```
make docker
```
Run the Docker image with:
```
docker run -v $(pwd)/collector-config.yaml:/etc/otel/config.yaml otelcollector-castai:latest
```
### Docker Compose to test sending logs to Loki 
Docker Compose exposes the following Grafana with Loki backend at http://0.0.0.0:3000

```
docker-compose -f docker-compose.yaml up -d
```

## Community

- [Twitter](https://twitter.com/cast_ai)
- [Discord](https://discord.gg/4sFCFVJ)

## License

Code is licensed under the [Apache License 2.0](LICENSE). See [NOTICE.md](NOTICE.md) for complete details, including software and third-party licenses and permissions.
