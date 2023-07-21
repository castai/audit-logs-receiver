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

.PHONY: build-and-run - Run a default collector that outputs everything to console
build-and-run: build run

.PHONY: docker - Build docker image and storing it locally
docker: BUILD_ARGS:=GOOS=linux
docker: build
	docker build -t castai-collector .

.PHONY: run-docker - Launch local docker image
run-docker: docker
	docker run castai-collector:latest

.PHONY: run-loki - Run a collector that exports Audit Logs to Grafana Loki
run-loki:
	./castai-collector/castai-collector --config ./examples/loki/collector-config.yaml

.PHONY: start-loki - Start Grafana Loki via docker compose
start-loki:
	docker-compose -f examples/loki/docker-compose.yaml up -d 

.PHONY: run-loki-demo - Run a collector that exports Audit Logs to Grafana Loki deployed with docker-compose
run-loki-demo: start-loki run-loki

.PHONY: run-coralogix - Run a collector that exports Audit Logs to Grafana Loki
run-loki:
	./castai-collector/castai-collector --config ./examples/coralogix/collector-config.yaml

.PHONY: run-file - Run a collector that exports Audit Logs to Grafana Loki
run-loki:
	./castai-collector/castai-collector --config ./examples/file/collector-config.yaml

.PHONY: build-and-run-loki - Build and run a collector that exports Audit Logs to Grafana Loki
build-and-run-loki: build run-loki

.PHONY: build-and-run-coralogix - Build and run a collector that exports Audit Logs to Coralogix
build-and-run-loki: build run-coralogix

.PHONY: build-and-run-file - Build and run a collector that exports Audit Logs to file
build-and-run-loki: build run-file
