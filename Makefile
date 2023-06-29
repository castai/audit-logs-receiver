update-collector:
	builder --skip-compilation --config cmd/otelcollector-castai/builder-config.yaml --output-path cmd/otelcollector-castai

build-collector:
	GOOS=linux go build -ldflags "-s -w" -o  bin/otelcollector-castai ./cmd/otelcollector-castai
	
push-local-docker:
	cp ./bin/otelcollector-castai ./cmd/otelcollector-castai
	docker build -t otelcollector-castai .

run-local-docker:
	docker run -v ./cmd/otelcollector-castai/config.yaml:/etc/otel/config.yaml otelcollector-castai:latest

build-local-docker: build-collector push-local-docker



