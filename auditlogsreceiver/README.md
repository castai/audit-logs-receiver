# CAST AI Receivers

This guide walks you through creating a custom TcpStats Receiver component for the [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/). We'll take a deep dive into the code and its integration within the larger OpenTelemetry Collector project.

The TcpStats Receiver collects TCP statistics from the `/proc/net/tcp` file on a Linux system and exposes them as metrics. We'll step through the code, pinpointing key concepts and walking you through implementation.

In this walkthrough, you'll gain knowledge about developing custom receivers for the OpenTelemetry Collector, understanding underlying mechanisms, and utilizing the power of OpenTelemetry to collect and export metrics.

Before we get started, ensure you have a basic understanding of Go programming and familiarity with the OpenTelemetry Collector project. Below, we overview the purpose and prerequisites of this tutorial.

*Note: This tutorial assumes you have a Go programming development environment set up with the necessary dependencies. The parent project includes a Dev Container with all dependencies included.*

# Purpose

The aim of this walkthrough is to impart a comprehensive understanding of developing a custom TcpStats Receiver for the OpenTelemetry Collector. You'll learn how to:

- Create a custom receiver for the OpenTelemetry Collector.
- Collect TCP statistics from the `/proc/net/tcp` file on a Linux system.
- Construct and emit metrics from the collected TCP statistics.
- Configure and validate the TcpStats Receiver.
- Generate the necessary metrics API using metadata.

We'll demystify implementation details throughout the tutorial and give you the knowledge to extend the capabilities of the OpenTelemetry Collector with custom receivers.

# Prerequisites

The parent project includes tools for building and testing an OpenTelemetry Collector including this custom receiver.

Before proceeding, ensure the following prerequisites:

1. **Go Programming Language**: The project requires Go 1.19 or later. If you haven't installed Go yet, follow the official Go installation instructions: [https://golang.org/doc/install](https://golang.org/doc/install)

2. **Makefile and Dependencies**: The parent project includes a Makefile that streamlines the build process and handles dependencies. If you're using the provided Makefile, run the `setup` target in the project directory to install necessary dependencies:

   ```shell
   make setup
   ```

3. **Visual Studio Code Dev Container (Optional)**: The parent project includes a Visual Studio Code Dev Container configuration. If you prefer using this, make sure you have:

   - [Visual Studio Code](https://code.visualstudio.com/) installed.
   - [Remote - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension installed in Visual Studio Code.
   - Docker installed to run the Dev Container.

   If using the Dev Container, open the project folder in Visual Studio Code and follow the prompts to reopen the project in the Dev Container.

With these prerequisites in place, let's explore the custom TcpStats Receiver for the OpenTelemetry Collector.

# Code Overview

This section gives an overview of the main components and files in the TcpStats Receiver implementation for the OpenTelemetry Collector. Understanding the code structure will help you follow the walkthrough and comprehend each component's functionality.

The code is organized into several files, each serving a specific purpose:

- [**factory.go**](./factory.go): Contains `NewFactory()` which returns a factory for TcpStats Receiver instances. Also defines `CreateTcpStatsReceiver()` for creating and initializing the receiver instance.

- [**scraper.go**](./scraper.go): Contains the `scraper` struct and its associated methods. The `scraper` collects TCP statistics and builds metrics based on the data.

- [**tcpstats.go**](./tcpstats.go): Contains the `tcpStats` struct and its associated methods. The `tcpStats` struct reads and parses the `/proc/net/tcp` file, extracting relevant TCP statistics.

- [**config.go**](./config.go): Defines the configuration structure (`Config`) for the TcpStats Receiver. It also includes functions for creating a default configuration and validating the provided configuration.

- [**metadata.yaml**](./metadata.yaml): Used by the `mdatagen` tool to generate the metrics API. It defines the attributes and metrics specific to the TcpStats Receiver.

We'll explore each file in detail to understand their collective contribution to a functional TcpStats Receiver.

Each of these files has a corresponding test file for unit testing: [`factory_test.go`](./factory_test.go), [`scraper_test.go`](./scraper_test.go), [`tcpstats_test.go`](./tcpstats_test.go), and [`config_test.go`](./config_test.go). These tests utilize data in the [`testdata`](./testdata/) folder. 

# Dependencies

The TcpStats Receiver relies on these dependencies:

1. **Go Modules**: For managing dependencies and versioning.

2. **OpenTelemetry Collector Libraries**: For integration with the Collector framework. The libraries provide interfaces and utilities for custom receivers:

   - `go.opentelemetry.io/collector/component`: Provides interfaces and structures for creating components, including receivers.

   - `go.opentelemetry.io/collector/consumer`: Defines consumer interfaces used for processing collected metrics.

   - `go.opentelemetry.io/collector/receiver`: Contains the receiver interface and related components.

   - `go.opentelemetry.io/collector/receiver/scraperhelper`: Offers helper functions and structures for building scrapers.

3. **Zap Logging Library**: For logging events and errors.

4. **Metadata Generation Tool**: The `mdatagen` tool processes the `metadata.yaml` file to generate necessary code for metrics.

With these dependencies, the TcpStats Receiver can be built and integrated into the OpenTelemetry Collector framework. 

# Factory and Receiver Creation

The TcpStats Receiver employs the OpenTelemetry Collector's component model to generate receiver instances. Let's look at the `factory.go` file, defining the factory function and receiver creation process.

## Factory Function

The `NewFactory()` function in `factory.go` gives a receiver factory, responsible for creating TcpStats Receiver instances:

```go
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(CreateTcpStatsReceiver, component.StabilityLevelDevelopment),
	)
}
```

`NewFactory()` yields a `receiver.Factory` interface, defined by the OpenTelemetry Collector, and takes three arguments:

1. `metadata.Type`: The TcpStats Receiver's type identifier (`tcpstatsreceiver`), used for registration within the Collector.

2. `createDefaultConfig`: Returns the default configuration for the TcpStats Receiver.

3. `receiver.WithMetrics(CreateTcpStatsReceiver, component.StabilityLevelDevelopment)`: Configures the receiver to collect and export metrics. It identifies `CreateTcpStatsReceiver` as the receiver creation function and sets the stability level to `component.StabilityLevelDevelopment`.

## Receiver Creation Function

`CreateTcpStatsReceiver()` is accountable for creating and initializing TcpStats Receiver instances:

```go
func CreateTcpStatsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	// ...
}
```

`CreateTcpStatsReceiver()` takes four parameters:

- `_ context.Context`: A context, useful for cancellation or value propagation.

- `settings receiver.CreateSettings`: Contains settings related to receiver creation, such as logger and resource detector.

- `cc component.Config`: Represents the TcpStats Receiver specific configuration.

- `consumer consumer.Metrics`: An interface allowing the receiver to push collected metrics to the OpenTelemetry Collector's metric pipeline.

The function returns a `receiver.Metrics` interface and an error, representing the created receiver instance and any issues during receiver creation, respectively.

# Scraper Implementation

The TcpStats Receiver uses a scraper to gather TCP statistics and create metrics from the collected data. Let's inspect the `scraper.go` file, outlining the scraper and its associated methods.

## Scraper Struct

The `scraper` struct encapsulates the scraper's functionality:

```go
type scraper struct {
	logger         *zap.Logger              
	metricsBuilder *metadata.MetricsBuilder 
	tcpStats       *tcpStats                
}
```

`scraper` struct includes:

- `logger`: A Zap logger reference for logging events and errors.

- `metricsBuilder`: An instance of the `MetricsBuilder` struct from the `metadata` package, responsible for building metrics from the collected TCP statistics.

- `tcpStats`: An instance of the `tcpStats` struct, responsible for reading and parsing the `/proc/net/tcp` file to extract TCP statistics.

## Scraper Initialization

`newScraper()` initializes a scraper instance:

```go
func newScraper(metricsBuilder *metadata.MetricsBuilder, path string, portFilter string, logger *zap.Logger) *scraper {
	return &scraper{
		logger:         logger,
		metricsBuilder: metricsBuilder,
		tcpStats:       newTcpStats(path, portFilter, logger),
	}
}
```

`newScraper()` generates a `scraper` struct instance and initializes its fields.

## Scrape Method

The `scrape()` method within the `scraper` struct collects TCP statistics and constructs metrics:

```go
func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	// ...
}
```

`scrape()` accepts a context and yields a

 `pmetric.Metrics` instance and an error. It follows these steps:

1. Logs the start of the scraping process.

2. Invokes the `get()` method of the `tcpStats` struct to retrieve TCP statistics from the `/proc/net/tcp` file.

3. Iterates over the collected statistics, using the `metricsBuilder` to record and build metrics.

4. Emits the metrics using the `metricsBuilder`.

5. Logs the completion of the scraping process.

# Parsing TCP Stats

TcpStats Receiver employs the `tcpStats` struct to parse the `/proc/net/tcp` file and extract TCP statistics. Let's inspect the `tcpstats.go` file, containing the `tcpStats` struct and its associated methods.

## tcpStats Struct

The `tcpStats` struct holds the functionality to read and parse the `/proc/net/tcp` file and extract relevant TCP statistics:

```go
type tcpStats struct {
	path       string
	portFilter map[int64]bool
	logger     *zap.Logger
}
```

`tcpStats` struct contains:

- `path`: The `/proc/net/tcp` file path to read and parse TCP statistics.

- `portFilter`: A map of port numbers to filter the collected statistics. The ports in this map will be included, while others will be discarded.

- `logger`: A Zap logger reference for logging events and errors.

## tcpStats Initialization

`newTcpStats()` initializes a `tcpStats` instance:

```go
func newTcpStats(path string, portFilter string, logger *zap.Logger) *tcpStats {
	return &tcpStats{
		path:       path,
		portFilter: parsePortFilter(portFilter, logger),
		logger:     logger,
	}
}
```

`newTcpStats()` creates a `tcpStats` struct instance and initializes its fields.

## Parsing the /proc/net/tcp File

`get()` method of the `tcpStats` struct reads and parses the `/proc/net/tcp` file to extract TCP statistics:

```go
func (t *tcpStats) get() ([]tcpStatsResult, error) {
	// ...
}
```

`get()` returns an array of `tcpStatsResult` structs and an error. It performs these steps:

1. Opens the `/proc/net/tcp` file for reading.

2. Parses the file line by line with a scanner.

3. Skips the first line (header line) of the file.

4. Parses each subsequent line to extract information, such as local address, local port, queues, and status.

5. Applies the port filter if provided, and skips statistics for ports not included in the filter.

6. Collects statistics into a map with unique combinations of local address and port as keys, and associated statistics as values.

7. Converts the map into an array of `tcpStatsResult` structs and returns it.

Next sections will delve into how the extracted TCP statistics are used to build metrics using `metricsBuilder` and explore configuration options available for the TcpStats Receiver.

# Building Metrics

TcpStats Receiver collects TCP statistics from the `/proc/net/tcp` file and builds metrics from them. `metricsBuilder` instance within the `scraper` struct handles the metrics building process. Let's explore how metrics are built based on the collected TCP statistics.

## MetricsBuilder Initialization

`metricsBuilder` is an instance of `MetricsBuilder` struct from the `metadata` package. It's initialized in the `newScraper()` function in the `scraper.go` file. `metricsBuilder` constructs metrics based on the collected TCP statistics and the given configuration.

## Metrics Building Process

`scrape()` method within the `scraper` struct orchestrates the metrics building process using the `metricsBuilder`:

1. Logs the start of the metrics building process.

2. Calls the `get()` method of `tcpStats` struct to retrieve TCP statistics.

3. Iterates over the collected statistics and employs `metricsBuilder` to record and build metrics based on the collected data.
   - `metricsBuilder` offers methods like `RecordTCPQueueSizeDataPoint()` and `RecordTCPQueueLengthDataPoint()` to record relevant TCP statistics as data points for the metrics.

4. Emits the built metrics using `metrics

Builder`'s `Emit()` method.

5. Logs the completion of the metrics building process.

This process includes iterating over the collected TCP statistics, creating data points for relevant metrics, and populating those data points with the extracted values. `metricsBuilder` encapsulates the logic for constructing and organizing the metrics.

Metrics are constructed based on the `metadata.yaml` file, defining attributes and metrics specific to TcpStats Receiver. This file is processed by the `mdatagen` tool to generate the metrics API, allowing `metricsBuilder` to create metrics conforming to the defined schema.

# Configuration

TcpStats Receiver offers configuration options for customization to fit specific requirements. This section will cover the available configuration options and their usage.

## Config Structure

TcpStats Receiver's configuration is defined by the `Config` struct in `config.go`:

```go
type Config struct {
	Path                                    string                   `mapstructure:"path"`
	PortFilter                              string                   `mapstructure:"portfilter"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
}
```

`Config` struct fields:

- `Path`: Specifies the path to the file to be scraped for TCP statistics. The default is `"/proc/net/tcp"`.

- `PortFilter`: Allows a comma-separated list of ports to filter the collected statistics. Only statistics for these ports will be included.

- `scraperhelper.ScraperControllerSettings`: Incorporates the settings for the scraper controller that controls the scraping interval. It inherits from the `ScraperControllerSettings` struct in the `scraperhelper` package.

- `metadata.MetricsBuilderConfig`: Includes the configuration options for `metricsBuilder` to enable or disable specific metrics. It inherits from the `MetricsBuilderConfig` struct in the `metadata` package.

## Default Configuration

TcpStats Receiver provides a default configuration with sensible defaults for various parameters. The `createDefaultConfig()` function in `config.go` returns the default configuration:

```go
func createDefaultConfig() component.Config {
	return &Config{
		Path:                      defaultPath,
		PortFilter:                "",
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}
```

`createDefaultConfig()` initializes a `Config` struct instance with default values, which can be customized to fit specific needs.

## Configuration Validation

The `Config` struct includes a `Validate()` method that validates the given configuration, ensuring its correctness and fulfilling necessary requirements. An error will be returned if any invalid settings or issues are detected.

# Generating Metrics API

TcpStats Receiver uses the `metadata.yaml` file and the `mdatagen` tool to generate metrics based on the collected TCP statistics. Let's examine this process.

## metadata.yaml File

The `metadata.yaml` file contains attribute and metric definitions specific to the TcpStats Receiver. It outlines the structure and properties of the metrics to be built based on the collected TCP statistics.

The `metadata.yaml` file includes:

- `type`: Defines the type of the TcpStats Receiver (`tcpstats`).

- `status`: Provides status information about the receiver, such as stability level and distribution type.

- `attributes`: Defines the attributes associated with the metrics, including a description, type, and `enabled` flag indicating inclusion in the metrics.

- `metrics`: Defines the metrics built based on the collected TCP statistics, including a description, a unit, a value type, and a list of associated attributes.

## Generating the Metrics API

The `mdatagen` tool processes the `metadata.yaml` file, generating the necessary code for the metrics API, including structures, methods, and utilities for handling and manipulating the metrics.

To generate the metrics API, use the following command:

```shell
mdatagen metadata.yaml 
```

After running `mdatagen`, the generated code can be found in the [internal/metadata](internal/metadata/) directory. This code includes necessary structures and methods to handle the metrics defined in `metadata.yaml`.

The Makefile in the parent project contains a target to generate these files when `metadata.yaml` is updated. 

With the generated metrics API, the `metricsBuilder` in the TcpStats Receiver can create and manipulate metrics based on the collected TCP statistics, adhering to the defined schema.

# Conclusion

This guide has taken you through the implementation of the TcpStats Receiver for the OpenTelemetry Collector. This receiver is designed to collect TCP statistics and build metrics from the gathered data. The discussion has included crucial components such as the factory function, receiver creation, scraper implementation, TCP stats parsing, metric building, and configuration options.

This guide should have given you a comprehensive understanding of creating a custom receiver for the OpenTelemetry Collector, leveraging scrapers, handling TCP statistics, and constructing metrics. Now, you are well-prepared to tailor the TcpStats Receiver to meet specific use cases and incorporate it into your larger project.

The TcpStats Receiver serves as a valuable starting point for further exploration and development of custom receivers and integrations within the OpenTelemetry ecosystem.

Happy coding and metrics collecting with the TcpStats Receiver!

# References

The development of the TcpStats Receiver and this guide involved referencing several resources:

1. [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/): The official documentation for the OpenTelemetry Collector, a valuable source of information on the architecture, concepts, and usage of the Collector.

2. [OpenTelemetry Collector GitHub Repository](https://github.com/open-telemetry/opentelemetry-collector): The OpenTelemetry Collector's GitHub repository, which contains source code and examples that were referred to during the TcpStats Receiver's implementation.

3. [Go Documentation](https://golang.org/doc/): The official documentation for the Go programming language, used to understand Go language constructs and standard library packages.

4. [Zap Logger Documentation](https://pkg.go.dev/go.uber.org/zap): The documentation for the Zap logger, a prominent logging library for Go, was utilized to comprehend its usage and capabilities.

Please be aware that the above references may be updated or changed, so always refer to the official documentation and resources for the most recent information.
