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
- Several Open Telemetry examples with different destinations (file, Grafana Loki, Coralogix, raw JSON in stdout)


### Setting things up

CAST AI Audit Logs receiver is not part of ['standard' receivers provided by Open Telemetry hosted here](https://github.com/open-telemetry/opentelemetry-collector-contrib).
So it requires building a custom Open Telemetry Collector (a program that combines selected receivers, processors and exporters into a pipeline used for pushing logs / metrics / traces).

The first step in building a custom Collector is installing required tools, which can be done as simple as running(the only prerequisite is having [Go](https://golang.org/doc/install) installed):
```
make setup
```

It installs:
- [Open Telemetry Metadata Generator](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/mdatagen) -
  required to generate receiver's definition (metadata about receiver itself); for example, stability level, is this a logs or metrics receiver, etc. Audit Logs Exporter's [metadata is defined here](./auditlogsreceiver/metadata.yaml).
- [Open Telemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) -
  required to generate a code that bootstraps selected components so compilation may produce an executable binary. Builder's [configuration is defined here](./builder-config.yaml)

Collector can be customized (what gets included in a binary artifact) as needed by tailoring `builder-config.yaml`.
Refer to OpenTelemetry Collector Contrib Distro's (for example, [the manifest](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/main/distributions/otelcol-contrib/manifest.yaml) for a full list of available components.

### Building and running an executable artifact

Building a custom Collector is as simple as:
```
make build
```

It produces few artifacts (including a binary executable file) into `castai-collector` directory.
Before running the Collector, it is required to set `CASTAI_API_URL` and `CASTAI_API_KEY` environment variables or provide them directly in `collector-config.yaml` file.
To run the newly built binary, use:
```
CASTAI_API_URL=https://api.cast.ai CASTAI_API_KEY=<api_access_key> ./castai-collector/castai-collector --config collector-config.yaml
```

It can also be executed by using a make target:
```
CASTAI_API_URL=https://api.cast.ai CASTAI_API_KEY=<api_access_key> make run
```

### Building and running as Docker container
Both building and running are support by Make targets and can be run as:
```
CASTAI_API_URL=https://api.cast.ai CASTAI_API_KEY=<api_access_key> make docker run-docker
```

There is one additional Make target to start Grafana with Loki (available via http://0.0.0.0:3000),
which may be useful if logs are exported to this destination.
In this scenario, one would start Loki first before running custom Collector:
```
make run-loki-server
```

### Helm Chart Support
A custom collector with Audit Logs receiver may be hosted on Kubernetes,
so to facilitate that a Helm Chart is published in [castai/helm-charts](https://github.com/castai/helm-charts).

One important aspect of hosting this collector on Kubernetes is that it is deployed as StatefulSet and uses PersistentVolumeClaim for storing data about fetching Audit Logs.
This data is required to ensure that all Audit Logs are collected even in case when Collector's pod got restarted.

### Usage
[Helm](https://helm.sh) must be installed to use the charts.
Please refer to Helm's [documentation](https://helm.sh/docs/) to get started.

Once Helm is set up properly, add [castai/helm-charts](https://github.com/castai/helm-charts) repository as follows:

```console
helm repo add castai-helm https://castai.github.io/helm-charts
```
To install Audit Logs receiver's release:
  * set `castai.apiKey` property to your CAST AI [API Access key](https://docs.cast.ai/docs/authentication#obtaining-api-access-key)
  * deploy the chart:
```shell
helm install logs-receiver castai-helm/castai-audit-logs-receiver \ 
  --namespace=castai-logs \
  --create-namespace \ 
  --set castai.apiKey=<api_access_key>
  --set castai.apiURL="https://api.cast.ai"
```
Default installation uses [logging](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/loggingexporter) as main log exporter but this can be changed by overriding chart's `config` property with desired collector's pipeline setup.  `collector-config.yaml` files for different exporter setups can be found in [examples](./examples/) directory for both reference and to create `values.yaml` file to pass to Helm chart.
Default image used in chart, which is `us-docker.pkg.dev/castai-hub/library/audit-logs-receiver`, is built with configuration from `builder-config.yaml` file from this repository. You can also build your own image with different extensions and exporters as described in previous sections and then override `image.repository` and `image.tag` properties in Helm chart.

Example Helm install with Loki configuration:
  * create your custom `values.yaml` with pipeline setup from [./examples/loki/collector-config.yaml](./examples/loki/collector-config.yaml):  
```shell
# values.yaml
config:
  exporters:
    loki:
      endpoint: http://localhost:3100/loki/api/v1/push

  processors:
    attributes: 
      actions:
        - action: insert
          key: loki.attribute.labels
          value: id, initiatedBy, eventType, labels.ClusterId

  service:
    pipelines:
      logs:
        receivers: [castai_audit_logs]
        processors: [attributes]
        exporters: [loki]
```
* deploy chart with `--values` flag set to `values.yaml`:
```shell
helm install logs-receiver castai-helm/castai-audit-logs-receiver \
  --namespace=castai-logs --create-namespace \
  --set castai.apiKey=<api_access_key>
  --set castai.apiURL="https://api.cast.ai" \
  --values values.yaml
```

To see all chart values that can be customized, run:
```shell
helm show values castai-helm/castai-audit-logs-receiver
```

## License

Code is licensed under the [Apache License 2.0](LICENSE). See [NOTICE.md](NOTICE.md) for complete details, including software and third-party licenses and permissions.
