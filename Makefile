.PHONY: setup - Set up required tools (builder, mdatagen)
setup:
	go install go.opentelemetry.io/collector/cmd/builder@latest
	go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen@latest

.PHONY: audit-logs-metadata - Generating Audit Logs receiver's metadata
audit-logs-metadata:
	cd auditlogsreceiver && mdatagen metadata.yaml

.PHONY: build - Generate and build collector
build: audit-logs-metadata
	$(BUILD_ARGS) builder --config builder-config.yaml

.PHONY: run - Run a default collector that outputs everything to console
run:
	./castai-collector/castai-collector --config collector-config.yaml

# =======================
# Docker related targets.
.PHONY: docker - Build docker image and storing it locally
docker: BUILD_ARGS:=GOOS=linux
docker: build
	docker build -t castai-collector .

.PHONY: run-docker - Launch local docker image
run-docker: docker
	docker run castai-collector:latest

# ==================================================
# Targets to run an example that uses file exporter.
.PHONY: run-file - Run a collector that exports Audit Logs to Grafana Loki
run-file:
	./castai-collector/castai-collector --config ./examples/file/collector-config.yaml

# ==================================================
# Targets to run an example that uses Loki exporter.
.PHONY: run-loki-server - Start Grafana Loki via docker compose
run-loki-server:
	docker-compose -f examples/loki/docker-compose.yaml up -d

.PHONY: run-loki - Run a collector that exports Audit Logs to Grafana Loki
run-loki:
	./castai-collector/castai-collector --config ./examples/loki/collector-config.yaml

# =======================================================
# Targets to run an example that uses Coralogix exporter.
.PHONY: run-coralogix - Run a collector that exports Audit Logs to Grafana Loki
run-coralogix:
	./castai-collector/castai-collector --config ./examples/coralogix/collector-config.yaml
