# opentelemetry-exporter-go

# The Honeycomb OpenTelemetry Exporter for Go

[![CircleCI](https://circleci.com/gh/honeycombio/opentelemetry-exporter-go.svg?style=svg)](https://circleci.com/gh/honeycombio/opentelemetry-exporter-go)

**NOTE:** Did you know that Honeycomb now supports OTLP ingest directly? This means you
can use [an OTLP exporter](https://docs.honeycomb.io/getting-data-in/go/opentelemetry-tutorial/#configure-export-to-honeycomb)
and no longer need this exporter to send data to Honeycomb.

## Default Exporter

The Exporter can be initialized using `sdktrace.WithSyncer`:

```golang
exporter, _ := honeycomb.NewExporter(
	honeycomb.Config{
		APIKey:  <YOUR-API-KEY>,
	},
	honeycomb.TargetingDataset(<YOUR-DATASET>),
	honeycomb.WithServiceName("example-server"),
	honeycomb.WithDebugEnabled()) // optional to output diagnostic logs to STDOUT

defer exporter.Shutdown(context.TODO())
sdktrace.NewProvider(
	sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	sdktrace.WithSyncer(exporter),
)
```

## Sampling

Read more about [sampling with Honeycomb in our docs](https://docs.honeycomb.io/working-with-your-data/tracing/sampling/).

## Example

You can find an example Honeycomb app in [/example](./example).
