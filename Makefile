.PHONY: collector - Generate and build collector
collector: audit-logs-metadata
	builder --config builder-config.yaml --output-path otelcollector

.PHONY: docker - Push image to local docker registry 
docker:
	GOOS=linux GOARCH=amd64 builder --config builder-config.yaml --output-path otelcollector-docker
	docker build -t otelcollector-castai .

.PHONY: run-docker - Launch local docker image
run-docker: docker
	docker run -v ./collector-config.yaml:/etc/otel/config.yaml otelcollector-castai:latest

.PHONY: setup - Set up used tools
setup: 
	go install go.opentelemetry.io/collector/cmd/builder@latest
	go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen@latest

.PHONY: audit-logs-metadata - Generating Audit Logs receiver's metadata
audit-logs-metadata:
	cd auditlogsreceiver && mdatagen metadata.yaml
