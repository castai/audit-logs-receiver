<a href="https://cast.ai">
    <img src="https://cast.ai/wp-content/themes/cast/img/cast-logo-dark-blue.svg" align="right" height="100" />
</a>

OpenTelemetry Collector Receiver for CAST AI
==================
Website: https://www.cast.ai

Custom OpenTelemetry Collector Receiver for collecting CAST AI Audit Logs

## Testing
### Build custom Collector with CAST AI receiver
```
make build-collector
```


## Update custom Collector
Install [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder):
```
go install go.opentelemetry.io/collector/cmd/builder@latest
```
Update configuration file  ```comd/otelcollector-castai/builder-config.yaml```.

Run the following command to generate the golang source code and get modules:

```
make update-collector
```

### Run locally with docker
```
make run-local-docker
```

### Local demo sending logs to Loki 
Demo uses local docker image ```otelcollector-castai:latest``` and exposes following Grafana with Loki backend at http://0.0.0.0:3000

```
make build-collector
docker-compose -f examples/demo/docker-compose.yaml up -d
```

## Community

- [Twitter](https://twitter.com/cast_ai)
- [Discord](https://discord.gg/4sFCFVJ)

## License

Code is licensed under the [Apache License 2.0](LICENSE). See [NOTICE.md](NOTICE.md) for complete details, including software and third-party licenses and permissions.∏