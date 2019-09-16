# opentelemetry-exporter-go

# The Honeycomb OpenTelemetry Exporter for Golang

[![CircleCI](https://circleci.com/gh/honeycombio/opentelemetry-exporter-go.svg?style=svg)](https://circleci.com/gh/honeycombio/opentelemetry-exporter-go)

## Default Exporter

The Exporter can be initialized as a default exporter:

```golang
exporter := honeycomb.NewExporter(<API_KEY>, <DATASET_NAME>)
exporter.ServiceName = "example-server"
defer exporter.Close()
exporter.Register()
```

## Sampling

The default exporter uses the OpenTelemetry Default Sampler `DefaultSampler: trace.AlwaysSample()` under the hood.

You can configure sampling with Honeycomb with either Deterministic Sampling or Dynamic Sampling.

Read more about [sampling with Honeycomb in our docs](https://docs.honeycomb.io/working-with-your-data/tracing/sampling/).
