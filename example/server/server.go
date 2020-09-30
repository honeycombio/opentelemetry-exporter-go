// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/api/baggage"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
		honeycomb.WithServiceName("opentelemetry-server"),
		honeycomb.WithDebugEnabled())
	if err != nil {
		log.Fatal(err)
	}
	defer exporter.Shutdown(context.Background())

	initTracer(exporter)

	tr := global.Tracer("honeycomb/example/server")

	helloHandler := func(w http.ResponseWriter, req *http.Request) {
		attrs, tags, spanCtx := otelhttptrace.Extract(req.Context(), req)

		req = req.WithContext(baggage.ContextWithMap(req.Context(), baggage.NewMap(baggage.MapUpdate{
			MultiKV: tags,
		})))

		ctx, span := tr.Start(
			trace.ContextWithRemoteSpanContext(req.Context(), spanCtx),
			"hello",
			trace.WithAttributes(attrs...),
			trace.WithLinks(trace.Link{SpanContext: spanCtx, Attributes: attrs}),
		)
		defer span.End()

		span.SetAttributes(label.String("ex.com/another", "yes"))
		span.AddEvent(ctx, "handling this...", label.Int("request-handled", 100))

		_, _ = io.WriteString(w, "Hello, world!\n")
	}

	http.HandleFunc("/hello", helloHandler)
	err = http.ListenAndServe(":7777", nil)
	if err != nil {
		log.Fatal(err)
	}
}
