// ADAPTED FROM OPENTELEMETRY BASIC EXAMPLE

package main

import (
	"context"
	"flag"
	"log"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/sdk/trace"
)

func main() {
	trace.Register()

	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	flag.Parse()

	exporter, err := honeycomb.NewExporter(honeycomb.Config{
		ApiKey:      *apikey,
		Dataset:     *dataset,
		Debug:       true,
		ServiceName: "opentelemetry-basic-example",
	})
	log.Fatal(err)

	defer exporter.Close()
	exporter.RegisterSimpleSpanProcessor()

	ctx := context.Background()
	ctx, span := apitrace.GlobalTracer().Start(ctx, "/foo")
	bar(ctx)
	span.End()
}

func bar(ctx context.Context) {
	_, span := apitrace.GlobalTracer().Start(ctx, "/bar")
	defer span.End()

	// Do bar...
}
