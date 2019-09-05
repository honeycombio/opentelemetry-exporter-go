// ADAPTEd FROM OPENTELEMETRY HTTP EXAMPLE

package main

import (
	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	"io"
	"net/http"

	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/api/tag"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/plugin/httptrace"
	"go.opentelemetry.io/sdk/trace"
)

func main() {
	trace.Register()

	exporter := honeycomb.NewExporter("API-KEY", "DATASET_NAME")
	exporter.ServiceName = "server"

	defer exporter.Close()
	trace.RegisterExporter(exporter)

	// For demoing purposes, always sample. In a production application, you should
	// configure this to a trace.ProbabilitySampler set at the desired
	// probability.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	tracer := apitrace.GlobalTracer().
		WithService("server").
		WithComponent("main").
		WithResources(
			key.New("whatevs").String("nooooo"),
		)

	helloHandler := func(w http.ResponseWriter, req *http.Request) {
		attrs, tags, spanCtx := httptrace.Extract(req)

		req = req.WithContext(tag.WithMap(req.Context(), tag.NewMap(tag.MapUpdate{
			MultiKV: tags,
		})))

		ctx, span := tracer.Start(
			req.Context(),
			"hello",
			apitrace.WithAttributes(attrs...),
			apitrace.ChildOf(spanCtx),
		)
		defer span.Finish()

		span.AddEvent(ctx, "handling this...")

		_, _ = io.WriteString(w, "Hello, world!\n")
	}

	http.HandleFunc("/hello", helloHandler)
	err := http.ListenAndServe(":7777", nil)
	if err != nil {
		panic(err)
	}
}
