// COPIED FROM OPENTELEMETRY HTTP EXAMPLE

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/otel/api/correlation"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/plugin/httptrace"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer(exporter *honeycomb.Exporter) {
	// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
	// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
	tp, err := sdktrace.NewProvider(sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
	if err != nil {
		log.Fatal(err)
	}
	global.SetTraceProvider(tp)
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
	defer exporter.Close()

	initTracer(exporter)

	tr := global.TraceProvider().Tracer("honeycomb/example/client")

	client := http.DefaultClient
	ctx := correlation.NewContext(context.Background(),
		kv.String("username", "donuts"),
	)

	var body []byte

	err = tr.WithSpan(ctx, "say hello",
		func(ctx context.Context) error {
			req, _ := http.NewRequest("GET", "http://localhost:7777/hello", nil)

			ctx, req = httptrace.W3C(ctx, req)
			httptrace.Inject(ctx, req)

			fmt.Printf("Sending request...\n")
			res, err := client.Do(req)
			if err != nil {
				panic(err)
			}
			body, err = ioutil.ReadAll(res.Body)

			res.Body.Close()
			trace.SpanFromContext(ctx).SetStatus(codes.OK, "")

			return err
		})

	if err != nil {
		panic(err)
	}

	fmt.Printf("%s", body)
}
