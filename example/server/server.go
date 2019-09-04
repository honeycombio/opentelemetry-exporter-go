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
	"github.com/davecgh/go-spew/spew"
	"github.com/honeycombio/opencensus-exporter/honeycomb"
	"io"
	"net/http"

	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/api/tag"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/plugin/httptrace"
	"go.opentelemetry.io/sdk/trace"

	_ "go.opentelemetry.io/experimental/streaming/exporter/stderr/install"
)

var (
	tracer = apitrace.GlobalTracer().
		WithService("server").
		WithComponent("main").
		WithResources(
			key.New("whatevs").String("nooooo"),
		)
)

func main() {
	exporter := honeycomb.NewExporter("44b49dc4ccb387b11a57fab6cf731c0c", "opentelemetry")
	spew.Dump("HELLO HONEYCOMB")
	defer exporter.Close()

	trace.RegisterExporter(exporter)

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
