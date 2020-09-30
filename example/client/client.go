// COPIED FROM OPENTELEMETRY CONTRIB HTTP EXAMPLE

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"

	otelhttptrace "go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/api/baggage"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
)

func initTracer(exporter *honeycomb.Exporter) {
	// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
	// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter),
	)
	global.SetTracerProvider(tp)
}

func main() {
	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	flag.Parse()

	exporter, err := honeycomb.NewExporter(
		honeycomb.Config{
			APIKey: *apikey,
		},
		honeycomb.TargetingDataset(*dataset),
		honeycomb.WithServiceName("opentelemetry-client"),
		honeycomb.WithDebugEnabled())
	if err != nil {
		log.Fatal(err)
	}
	defer exporter.Shutdown(context.Background())

	initTracer(exporter)

	url := flag.String("server", "http://localhost:7777/hello", "server url")
	flag.Parse()

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	ctx := baggage.NewContext(context.Background(),
		label.String("username", "donuts"),
	)

	ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))

	var body []byte

	tr := global.Tracer("example/client")
	err = func(ctx context.Context) error {
		ctx, span := tr.Start(ctx, "say hello", trace.WithAttributes(semconv.PeerServiceKey.String("ExampleService")))
		defer span.End()
		req, _ := http.NewRequestWithContext(ctx, "GET", *url, nil)

		fmt.Printf("Sending request...\n")
		res, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		body, err = ioutil.ReadAll(res.Body)
		_ = res.Body.Close()

		return err
	}(ctx)

	if err != nil {
		panic(err)
	}

	fmt.Printf("Response Received: %s\n\n\n", body)
	fmt.Printf("Waiting for few seconds to export spans ...\n\n")
	time.Sleep(10 * time.Second)
	fmt.Printf("Inspect traces on stdout\n")
}
