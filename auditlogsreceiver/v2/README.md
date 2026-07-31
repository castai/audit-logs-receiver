CAST AI Audit Logs V2 Receiver (Alpha)
=================

> **Status: Alpha** — This receiver is under active development and intended for early testing with friendly users. It is not production-ready and we may introduce breaking changes. The v1 receiver (`castai_audit_logs`) remains the supported version.

This receiver polls the CAST AI Audit API v2 and maps audit events to OpenTelemetry log records. It is built as a separate package in the same repository, and both v1 and v2 receivers can be included in the same collector binary.

Compared to v1, the v2 receiver supports:
- Richer event schema (hierarchical `domain.resource.action` event types, actor and resource entities, severity model, correlation IDs, labels)
- Seven filter dimensions (clusters, domains, resources, actions, sources, severity, search)
- Reliable checkpoint persistence
- Pagination that respects the poll interval

For general information about this repository — what an OpenTelemetry Collector is, how the build system works, and how to install the required tools — see the [main README](../../README.md).


### Setting things up

The setup steps are the same as for v1. Follow the [Setting things up](../../README.md#setting-things-up) section in the main README to install the required tools (`make setup`).

The v2 receiver is already included in [builder-config.yaml](../../builder-config.yaml), so `make build` produces a collector binary with both v1 and v2 receivers.

### Building and running an executable artifact

Building the collector:
```
make build
```

Before running the collector, set the `CASTAI_API_URL` and `CASTAI_API_KEY` environment variables. A minimal v2 config using the debug exporter is available at [collector-config-v2.yaml](../../collector-config-v2.yaml):
```
CASTAI_API_URL=https://api.cast.ai CASTAI_API_KEY=<api_access_key> \
  ./castai-collector/castai-collector --config collector-config-v2.yaml
```

### Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `api.url` | string | `https://api.cast.ai` | CAST AI API URL |
| `api.key` | string | *(required)* | CAST AI API access key |
| `api.timeout` | duration | `30s` | HTTP client timeout |
| `poll_interval` | duration | `10s` | Interval between poll cycles |
| `page_limit` | int | `100` | Max records per API page (1–250) |
| `lookback` | duration | `0s` | On first run (no checkpoint), fetch events from this far back. If omitted, starts from the current time. |
| `checkpoint_file` | string | *(empty)* | Path to checkpoint file. Empty = in-memory (lost on restart). Set to a file path for persistent checkpointing. |
| `filters.search` | string | *(empty)* | Full-text search query |
| `filters.clusters` | []string | *(empty)* | Filter by cluster IDs |
| `filters.domains` | []string | *(empty)* | Filter by event domains (e.g. `autoscaler`, `workload`) |
| `filters.resources` | []string | *(empty)* | Filter by event resources (e.g. `node`, `pod`) |
| `filters.actions` | []string | *(empty)* | Filter by event actions (e.g. `created`, `deleted`) |
| `filters.sources` | []string | *(empty)* | Filter by event sources |
| `filters.severity` | []string | *(empty)* | Filter by severity (`info`, `warn`, `error`) |

Example:
```yaml
receivers:
  castai_audit_logs_v2:
    api:
      url: ${env:CASTAI_API_URL}
      key: ${env:CASTAI_API_KEY}
    poll_interval: 10s
    page_limit: 100
    checkpoint_file: ./audit_logs_v2_checkpoint.json
    lookback: 1h
    filters:
      domains: [autoscaler, workload]
      resources: [node]
      actions: [deleted]
      severity: [error]

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    logs:
      receivers: [castai_audit_logs_v2]
      exporters: [debug]
```

### Attribute mapping

Each v2 audit event is mapped to an OpenTelemetry log record as follows:

| OTel field | Attribute key | Notes |
|---|---|---|
| Timestamp | — | Event occurrence time |
| ObservedTimestamp | — | Set when the receiver processes the event |
| Body | — | Human-readable event description |
| SeverityNumber | — | Mapped to OTel severity numbers (info=9, warn=13, error=17) |
| SeverityText | — | `info`, `warn`, or `error` |
| Attribute | `event.id` | UUID |
| Attribute | `event.domain` | e.g. `autoscaler`, `workload`, `kent` |
| Attribute | `event.resource` | e.g. `node`, `surge`, `recommended_requests` |
| Attribute | `event.action` | e.g. `created`, `updated`, `deleted` |
| Attribute | `actor.id` | e.g. `internal\|autoscaler`, `csp\|aws` |
| Attribute | `actor.type` | `internal`, `user`, or `csp` |
| Attribute | `actor.display_name` | |
| Attribute | `actor.email` | May be empty for internal/CSP actors |
| Attribute | `resource.type` | e.g. `Pod`, `Deployment`, `node` |
| Attribute | `resource.id` | UUID, may be empty |
| Attribute | `resource.display_name` | |
| Attribute | `cluster.id` | Only set when present |
| Attribute | `tenant.id` | |
| Attribute | `ingested_at` | RFC3339 timestamp |
| Attribute | `labels` | Nested map, varies by event type |
| Attribute | `correlation.id` | Only set when present |
| Attribute | `correlation.count` | Only set when `correlation.id` is present |
| Attribute | `request.id` | Only set when present |
| TraceID | — | If the correlation ID is a valid UUID, it is also set as the OTel TraceID |

### Examples

V2 example configs are available in the [examples](../../examples/) directory:

| Exporter | Config | Notes |
|----------|-------|-------|
| Debug (stdout) | [examples/stdout/collector-config-v2.yaml](../../examples/stdout/collector-config-v2.yaml) | File exporter to `/dev/stdout` |
| File | [examples/file/collector-config-v2.yaml](../../examples/file/collector-config-v2.yaml) | File exporter to a log file |
| Grafana Loki | [examples/loki/v2/](../../examples/loki/v2/) | OTLP HTTP to Loki, includes docker-compose and Loki config |
| Coralogix | [examples/coralogix/v2/](../../examples/coralogix/v2/) | Coralogix exporter |
| Datadog | [examples/datadog/v2/](../../examples/datadog/v2/) | Datadog exporter, includes Dockerfile and docker-compose |
| Splunk | [examples/splunk/v2/](../../examples/splunk/v2/) | Splunk HEC exporter, includes Dockerfile and docker-compose |
| Filtering | [examples/filtering/](../../examples/filtering/) | V1 and v2 filter examples (single-domain, multi-value combinations) |

### How to test locally

1. Build the collector:
   ```
   make build
   ```

2. Set environment variables:
   ```
   export CASTAI_API_URL=https://api.cast.ai
   export CASTAI_API_KEY=<api_access_key>
   ```

3. Run with the debug config:
   ```
   ./castai-collector/castai-collector --config collector-config-v2.yaml
   ```

   You should see log records printed to the console with all v2 attributes.

4. To test with a specific exporter, copy the relevant example config and adjust as needed. For a full local setup with Grafana Loki, see the [Loki v2 example](../../examples/loki/v2/):
   ```
   docker compose -f examples/loki/v2/docker-compose.yaml up -d
   ./castai-collector/castai-collector --config examples/loki/v2/collector-config.yaml
   ```

   Grafana will be available at http://localhost:3000.

### Checkpoint persistence

The receiver stores its polling position in a checkpoint file so it can resume without duplicates after a restart. To start fresh (e.g. to re-fetch recent events), delete the checkpoint file and restart the collector.

If no checkpoint file exists and `lookback` is set, the first poll fetches events from `now - lookback`. If no checkpoint and no lookback, the first poll starts from the current time.

### V1 to V2 migration notes

- The receiver type changes from `castai_audit_logs` to `castai_audit_logs_v2`
- `poll_interval_sec` (int, seconds) becomes `poll_interval` (duration, e.g. `10s`)
- `storage.type` + `storage.filename` becomes `checkpoint_file` (empty = in-memory, set = file)
- `filters.cluster_id` (single string) becomes `filters.clusters` ([]string)
- v2 adds six new filter dimensions: `domains`, `resources`, `actions`, `sources`, `severity`, `search`
- v1 attributes (`eventType`, `id`, `initiatedBy`, `labels.clusterId`) are replaced by v2 attributes (`event.domain` + `event.resource` + `event.action`, `event.id`, `actor.id`, `cluster.id`). See the [attribute mapping table](#attribute-mapping) for the full list.
