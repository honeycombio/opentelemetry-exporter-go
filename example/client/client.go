// COPIED FROM OPENTELEMETRY HTTP EXAMPLE

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/api/tag"
	"go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/plugin/httptrace"
)

var (
	tracer = trace.GlobalTracer().
		WithService("client").
		WithComponent("main").
		WithResources(
			key.New("whatevs").String("yesss"),
		)
)

func main() {
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

			trace.Inject(ctx, inj)

			res, err := client.Do(req)
			if err != nil {
				panic(err)
			}
			body, err = ioutil.ReadAll(res.Body)
			res.Body.Close()
			trace.CurrentSpan(ctx).SetStatus(codes.OK)

			return err
		})

	if err != nil {
		panic(err)
	}

	fmt.Printf("%s", body)
}
