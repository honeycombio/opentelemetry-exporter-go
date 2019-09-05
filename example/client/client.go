// ADAPTED FROM OPENTELEMETRY HTTP EXAMPLE

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/api/tag"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/plugin/httptrace"
	"go.opentelemetry.io/sdk/trace"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
)

func main() {
	trace.Register()
	exporter := honeycomb.NewExporter("API_KEY", "DATASET_NAME")
	exporter.ServiceName = "client"

	defer exporter.Close()
	trace.RegisterExporter(exporter)

	// For demoing purposes, always sample. In a production application, you should
	// configure this to a trace.ProbabilitySampler set at the desired
	// probability.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	tracer := apitrace.GlobalTracer().
		WithService("client").
		WithComponent("main").
		WithResources(
			key.New("whatevs").String("yesss"),
		)

	fmt.Printf("Tracer %v\n", tracer)
	client := http.DefaultClient
	ctx := tag.NewContext(context.Background(),
		tag.Insert(key.New("username").String("donuts")),
	)

	var body []byte

	err := tracer.WithSpan(ctx, "say hello",
		func(ctx context.Context) error {
			req, _ := http.NewRequest("GET", "http://localhost:7777/hello", nil)

			ctx, req, inj := httptrace.W3C(ctx, req)

			apitrace.Inject(ctx, inj)

			res, err := client.Do(req)
			if err != nil {
				panic(err)
			}
			body, err = ioutil.ReadAll(res.Body)

			res.Body.Close()
			apitrace.CurrentSpan(ctx).SetStatus(codes.OK)

			return err
		})

	if err != nil {
		panic(err)
	}

	fmt.Printf("%s", body)
}
