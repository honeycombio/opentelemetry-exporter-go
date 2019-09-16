// ADAPTED FROM OPENTELEMETRY BASIC EXAMPLE

package main

import (
	"context"
	"flag"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	apitrace "go.opentelemetry.io/api/trace"
)

func main() {
	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	flag.Parse()

	exporter := honeycomb.NewExporter(honeycomb.Config{
		ApiKey:      *apikey,
		Dataset:     *dataset,
		Debug:       true,
		ServiceName: "opentelemetry-basic-example",
	})

	defer exporter.Close()
	exporter.Register()

	ctx := context.Background()
	ctx, span := apitrace.GlobalTracer().Start(ctx, "/foo")
	bar(ctx)
	span.Finish()
}

func bar(ctx context.Context) {
	_, span := apitrace.GlobalTracer().Start(ctx, "/bar")
	defer span.Finish()

	// Do bar...
}
