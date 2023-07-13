.PHONY: setup - Set up required tools (builder, mdatagen)
setup:
	go install go.opentelemetry.io/collector/cmd/builder@latest
	go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen@latest

.PHONY: audit-logs-metadata - Generating Audit Logs receiver's metadata
audit-logs-metadata:
	cd auditlogsreceiver && mdatagen metadata.yaml

.PHONY: build - Generate and build collector
build: audit-logs-metadata
	$(BUILD_ARGS) builder --config builder-config-console.yaml

.PHONY: run - Run a default collector that outputs everything to console
run:
	./collector-console/castai-collector-console --config collector-console-config.yaml

.PHONY: build-and-run - Run a default collector that outputs everything to console
build-and-run: build run

.PHONY: docker - Build docker image and storing it locally
docker: BUILD_ARGS:=GOOS=linux
docker: build
	docker build -t castai-collector-console .

.PHONY: run-docker - Launch local docker image
run-docker: docker
	docker run castai-collector-console:latest

.PHONY: build - Generate and build collector meant to export Audit Logs to Grafana Loki
build-loki: audit-logs-metadata
	$(BUILD_ARGS) builder --config ./examples/loki/builder-config.yaml

.PHONY: run-loki - Run a collector that exports Audit Logs to Grafana Loki
run-loki:
	./examples/loki/collector-loki/castai-collector-loki --config ./examples/loki/collector-config.yaml

.PHONY: build-and-run-loki - Build and run a collector that exports Audit Logs to Grafana Loki
build-and-run-loki: build-loki run-loki

.PHONY: docker-loki - Build docker image and storing it locally
docker-loki: BUILD_ARGS:=GOOS=linux
docker-loki: build-loki
	docker build -t castai-collector-loki ./examples/loki/.

.PHONY: run-docker - Launch local docker image
run-docker-loki: docker-loki
	docker run castai-collector-loki:latest
