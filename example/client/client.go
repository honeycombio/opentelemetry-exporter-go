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

	"go.opentelemetry.io/api/distributedcontext"
	"go.opentelemetry.io/api/key"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/plugin/httptrace"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
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
		ServiceName: "opentelemetry-client",
	})
	if err != nil {
		log.Fatal(err)
	}

	defer exporter.Close()
	exporter.RegisterSimpleSpanProcessor()

	client := http.DefaultClient
	ctx := distributedcontext.NewContext(context.Background(),
		distributedcontext.Insert(key.New("username").String("donuts")),
	)

	var body []byte

	err = apitrace.GlobalTracer().WithSpan(ctx, "say hello",
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
			apitrace.CurrentSpan(ctx).SetStatus(codes.OK)

			return err
		})

	if err != nil {
		panic(err)
	}

	fmt.Printf("%s", body)
}
