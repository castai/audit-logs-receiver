GOARCH := $(shell go env GOARCH)

.PHONY: setup # Set up required tools (builder, mdatagen)
setup:
	git clone https://github.com/open-telemetry/opentelemetry-collector.git ./opentelemetry-collector
	cd ./opentelemetry-collector && git checkout v0.147.0
	cd ./opentelemetry-collector/cmd/builder && go build -o builder .
	cd ./opentelemetry-collector/cmd/mdatagen && go build -o mdatagen .

.PHONY: audit-logs-metadata # Generating Audit Logs receiver's metadata
audit-logs-metadata:
	cd auditlogsreceiver && ../opentelemetry-collector/cmd/mdatagen/mdatagen metadata.yaml

.PHONY: audit-logs-v2-metadata # Generating Audit Logs v2 receiver's metadata
audit-logs-v2-metadata:
	cd auditlogsreceiver/v2 && ../../opentelemetry-collector/cmd/mdatagen/mdatagen metadata.yaml
	# mdatagen derives the package name from the directory name ("v2"), but the
	# package is named "auditlogsreceiver" to match v1. Fix the generated files.
	sed -i.bak 's/^package v2$$/package auditlogsreceiver/' auditlogsreceiver/v2/generated_component_test.go auditlogsreceiver/v2/generated_package_test.go && rm -f auditlogsreceiver/v2/*.bak

.PHONY: build # Generate and build collector
build: audit-logs-metadata audit-logs-v2-metadata
	$(BUILD_ARGS) ./opentelemetry-collector/cmd/builder/builder --config builder-config.yaml

.PHONY: run # Run a default collector that outputs everything to console
run:
	./castai-collector/castai-collector --config collector-config.yaml

# =======================
# Docker related targets.
.PHONY: docker # Build docker image and storing it locally
docker: BUILD_ARGS:=GOOS=linux
docker: build
	cd castai-collector && GOOS=linux CGO_ENABLED=0 go build -o castai-collector-$(GOARCH)
	docker build -t castai-collector . 

.PHONY: run-docker # Launch local docker image
run-docker: docker
	docker run -e CASTAI_API_URL="$(CASTAI_API_URL)" -e CASTAI_API_KEY="$(CASTAI_API_KEY)" castai-collector:latest

# ==================================================
# Targets to run an example that uses file exporter.
.PHONY: run-file # Run a collector that exports Audit Logs to Grafana Loki
run-file:
	./castai-collector/castai-collector --config ./examples/file/collector-config.yaml

# ==================================================
# Targets to run an example that uses Loki exporter.
.PHONY: run-loki-server # Start Grafana Loki via docker compose
run-loki-server:
	docker-compose -f examples/loki/docker-compose.yaml up -d

.PHONY: run-loki # Run a collector that exports Audit Logs to Grafana Loki
run-loki:
	./castai-collector/castai-collector --config ./examples/loki/collector-config.yaml

# =======================================================
# Targets to run an example that uses Coralogix exporter.
.PHONY: run-coralogix # Run a collector that exports Audit Logs to Grafana Loki
run-coralogix:
	./castai-collector/castai-collector --config ./examples/coralogix/collector-config.yaml

.PHONY: build-datadog
build-datadog: BUILD_ARGS:=GOOS=linux
build-datadog: build
	# Build for multiple architectures
	cd castai-collector && \
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o castai-collector-amd64 && \
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o castai-collector-arm64

.PHONY: run-datadog-docker
run-datadog-docker: build-datadog
	cd examples/datadog && docker compose up --build

.PHONY: build-splunk
build-splunk: BUILD_ARGS:=GOOS=linux
build-splunk: build
	# Build for multiple architectures
	cd castai-collector && \
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o castai-collector-amd64 && \
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o castai-collector-arm64

.PHONY: run-splunk-docker
run-splunk-docker: build-splunk
	cd examples/splunk && docker compose up --build
