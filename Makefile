.PHONY: collector - Generate and build collector
build: audit-logs-metadata
	builder --config builder-config.yaml

.PHONY: build-collector-console - Generate and compile a default collector that outputs everything to console
build-collector-console: audit-logs-metadata
	builder --config builder-config-console.yaml

.PHONY: build-collector-console - Run a default collector that outputs everything to console
run-collector-console:
	./collector-console/castai-collector-console --config collector-console-config.yaml

.PHONY: build-collector-console - Generate, compile and run a default collector that outputs everything to console
build-run-collector-console: build-collector-console run-collector-console

.PHONY: docker - Push image to local docker registry
docker:
	GOOS=linux GOARCH=amd64 builder --config builder-config.yaml --output-path otelcollector-docker
	docker build -t otelcollector-castai .

.PHONY: run-docker - Launch local docker image
run-docker: docker
	docker run -v ./collector-config.yaml:/etc/otel/config.yaml otelcollector-castai:latest

.PHONY: setup - Set up required tools (builder, mdatagen)
setup: 
	go install go.opentelemetry.io/collector/cmd/builder@latest
	go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen@latest

.PHONY: audit-logs-metadata - Generating Audit Logs receiver's metadata
audit-logs-metadata:
	cd auditlogsreceiver && mdatagen metadata.yaml
