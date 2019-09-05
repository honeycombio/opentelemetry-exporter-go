// ADAPTED FROM OPENTELEMETRY BASIC EXAMPLE

package main

import (
	"context"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/sdk/trace"
)

func main() {
	trace.Register()
	ctx := context.Background()

	// Register the Honeycomb exporter to be able to retrieve
	// the collected spans.
	exporter := honeycomb.NewExporter("API_KEY", "DATASET_NAME")
	exporter.ServiceName = "opentelemetry-baseic-example"

	defer exporter.Close()
	trace.RegisterExporter(exporter)

	// For demoing purposes, always sample. In a production application, you should
	// configure this to a trace.ProbabilitySampler set at the desired
	// probability.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	ctx, span := apitrace.GlobalTracer().Start(ctx, "/foo")
	bar(ctx)
	span.Finish()

	// exporter.Flush()
}

func bar(ctx context.Context) {
	_, span := apitrace.GlobalTracer().Start(ctx, "/bar")
	defer span.Finish()

	// Do bar...
}
