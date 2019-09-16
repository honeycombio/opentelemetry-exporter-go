// COPIED FROM OPENTELEMETRY HTTP EXAMPLE

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/api/tag"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/plugin/httptrace"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
)

func main() {
	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	flag.Parse()

	exporter := honeycomb.NewExporter(*apikey, *dataset)
	exporter.ServiceName = "opentelemetry-client"
	defer exporter.Close()
	exporter.Register()

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
