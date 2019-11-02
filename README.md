# opentelemetry-exporter-go

# The Honeycomb OpenTelemetry Exporter for Golang

[![CircleCI](https://circleci.com/gh/honeycombio/opentelemetry-exporter-go.svg?style=svg)](https://circleci.com/gh/honeycombio/opentelemetry-exporter-go)

## Default Exporter

The Exporter can be initialized using `sdktrace.WithSyncer`:

```golang
exporter, _ := honeycomb.NewExporter(honeycomb.Config{
    ApiKey:  <YOUR-API-KEY>,
    Dataset: <YOUR-DATASET>,
    Debug:   true, // optional to output to stdout
    ServiceName: "example-server",
})

defer exporter.Close()
sdktrace.NewProvider(sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
```

## Sampling

Read more about [sampling with Honeycomb in our docs](https://docs.honeycomb.io/working-with-your-data/tracing/sampling/).

## Example

You can find an example Honeycomb app in [/example](./example)
