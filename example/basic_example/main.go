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

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/sdk/trace"
)

func main() {
	trace.Register()
	ctx := context.Background()

	// Register the Honeycomb exporter to be able to retrieve
	// the collected spans.
	exporter := honeycomb.NewExporter("API-KEY", "dataset-name")
	exporter.ServiceName = "opentelemetry-example"

	defer exporter.Close()
	trace.RegisterExporter(exporter)

	// For demoing purposes, always sample. In a production application, you should
	// configure this to a trace.ProbabilitySampler set at the desired
	// probability.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	ctx, span := apitrace.GlobalTracer().Start(ctx, "/foo")
	bar(ctx)
	span.Finish()

	// exporter.Flush()
}

func bar(ctx context.Context) {
	_, span := apitrace.GlobalTracer().Start(ctx, "/bar")
	defer span.Finish()

	// Do bar...
}
