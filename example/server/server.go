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
	"flag"
	"io"
	"net/http"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/api/tag"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/plugin/httptrace"
	"go.opentelemetry.io/sdk/trace"
)

func main() {
	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	flag.Parse()

	exporter := honeycomb.NewExporter(*apikey, *dataset)
	exporter.ServiceName = "opentelemetry-server"
	defer exporter.Close()
	exporter.Register()

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

		span.AddEvent(ctx, "handling this...", key.New("request-handled").Int(100))

		_, _ = io.WriteString(w, "Hello, world!\n")
	}

	http.HandleFunc("/hello", helloHandler)
	err := http.ListenAndServe(":7777", nil)
	if err != nil {
		panic(err)
	}
}
