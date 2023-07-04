<a href="https://cast.ai">
    <img src="https://cast.ai/wp-content/themes/cast/img/cast-logo-dark-blue.svg" align="right" height="100" />
</a>

OpenTelemetry Collector Receiver for CAST AI
==================
Website: https://www.cast.ai

Custom OpenTelemetry Collector Receiver for collecting CAST AI Audit Logs

## Testing


CAST AI Collector's distribution is generated using [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) with 
configuration taken from ```builder-config.yaml```.


### Install Dependencies
Run:
```
make setup
```
This installs `builder@latest` and `mdatagen@latest`.

Adjust disctribution's components in `builder-config.yaml` if needed.
(Refer to OpenTelemetry Collector Contrib Distro's [manifest](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/main/distributions/otelcol-contrib/manifest.yaml) for full list of available components).

### Build Controller's binary
```
make collector
```
To run the newly built binary, use:
```
./otelcollector-castai -config collector-config.yaml
```

### Build and Launch the Docker Image
Build the Docker image with:
```
make docker
```
Run the Docker image with:
```
docker run -v $(pwd)/collector-config.yaml:/etc/otel/config.yaml otelcollector-castai:latest
```
### Docker Compose to test sending logs to Loki 
Docker Compose exposes following Grafana with Loki backend at http://0.0.0.0:3000

```
docker-compose -f docker-compose.yaml up -d
```

## Community

- [Twitter](https://twitter.com/cast_ai)
- [Discord](https://discord.gg/4sFCFVJ)

## License

Code is licensed under the [Apache License 2.0](LICENSE). See [NOTICE.md](NOTICE.md) for complete details, including software and third-party licenses and permissions.