// ADAPTED FROM OPENTELEMETRY BASIC EXAMPLE

package main

import (
	"context"
	"flag"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/sdk/trace"
)

func main() {
	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	flag.Parse()

	trace.Register()
	exporter := honeycomb.NewExporter(*apikey, *dataset)
	exporter.ServiceName = "opentelemetry-basic-example"
	ctx := context.Background()

	defer exporter.Close()
	trace.RegisterExporter(exporter)

	// For demoing purposes, always sample. In a production application, you should
	// configure this to a trace.ProbabilitySampler set at the desired
	// probability.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	ctx, span := apitrace.GlobalTracer().Start(ctx, "/foo")
	bar(ctx)
	span.Finish()
}

func bar(ctx context.Context) {
	_, span := apitrace.GlobalTracer().Start(ctx, "/bar")
	defer span.Finish()

	// Do bar...
}
