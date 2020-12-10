// COPIED FROM OPENTELEMETRY CONTRIB HTTP EXAMPLE

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"go.opentelemetry.io/otel/trace"
)

func initTracer(exporter *otlp.Exporter) func(context.Context) error {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
	tp.ApplyConfig(
		// The default takes parent sampler hints into account, which we don't need here.
		sdktrace.Config{
			// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
			// In a production application, use sdktrace.ProbabilitySampler with a desired
			// probability.
			DefaultSampler: sdktrace.AlwaysSample(),
			Resource: resource.NewWithAttributes(
				label.String("service.name", "client"),
				label.String("service.version", "0.1"),
			),
		})
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{}))
	return bsp.Shutdown
}

func main() {
	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	flag.Parse()

	// exporter, err := honeycomb.NewExporter(
	// 	honeycomb.Config{
	// 		APIKey: *apikey,
	// 	},
	// 	honeycomb.TargetingDataset(*dataset),
	// 	honeycomb.WithServiceName("opentelemetry-client"),
	// 	honeycomb.WithDebugEnabled())

	exporter, err := otlp.NewExporter(
		otlp.WithInsecure(),
		otlp.WithAddress("api-dogfood.honeycomb.io:9090"),
		otlp.WithHeaders(map[string]string{
			"x-honeycomb-apikey": *apikey,
			"dataset":            *dataset,
		}),
	)

	if err != nil {
		log.Fatal(err)
	}
	defer exporter.Shutdown(context.Background())
	defer initTracer(exporter)(context.Background())
	tr := otel.Tracer("honeycomb/example/client")

	url := flag.String("server", "http://localhost:7777/hello", "server URL")
	flag.Parse()

	ctx := baggage.ContextWithValues(context.Background(),
		label.String("username", "donuts"))

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	ctx, span := tr.Start(ctx, "say hello",
		trace.WithAttributes(semconv.PeerServiceKey.String("ExampleService")))
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", *url, nil)
	if err != nil {
		panic(err)
	}
	_, req = otelhttptrace.W3C(ctx, req)

	fmt.Println("Sending request...")
	res, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HTTP request failed: %v\n", err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read HTTP response body: %v\n", err)
	}

	fmt.Printf("Response received (HTTP status code %d): %s\n\n\n", res.StatusCode, body)
}
