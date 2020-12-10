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
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func initTracer(exporter *otlp.Exporter) func(context.Context) error {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
	tp.ApplyConfig(
		// The default takes parent sampler hints into account, which we don't need here.
		sdktrace.Config{
			// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
			// In a production application, use sdktrace.ProbabilitySampler with a desired
			// probability.
			DefaultSampler: sdktrace.AlwaysSample(),
			Resource: resource.NewWithAttributes(
				label.String("service.name", "server"),
				label.String("service.version", "0.1"),
			),
		})
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{}))
	return bsp.Shutdown
}

func joinIPAddressAndPort(address net.IP, port string) string {
	var host string
	var empty net.IP
	if !address.Equal(empty) {
		host = address.String()
	}
	return net.JoinHostPort(host, port)
}

func runHTTPServer(address net.IP, port string, handler http.Handler, stop <-chan struct{}) error {
	server := &http.Server{
		Addr:    joinIPAddressAndPort(address, port),
		Handler: handler,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-stop
		// Don't bother imposing a timeout here.
		server.Shutdown(context.Background())
	}()
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	wg.Wait()
	return nil
}

func handleSignals(term func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	signal.Stop(c)
	term()
}

func main() {
	apikey := flag.String("apikey", "", "Your Honeycomb API Key")
	dataset := flag.String("dataset", "opentelemetry", "Your Honeycomb dataset")
	serverAddress := flag.String("server-address", "", "IP address on which to serve HTTP requests")
	serverPort := flag.String("server-port", "7777", "Port on which to serve HTTP requests")
	flag.Parse()

	var serverIPAddress net.IP
	if len(*serverAddress) > 0 {
		if serverIPAddress = net.ParseIP(*serverAddress); serverIPAddress == nil {
			log.Fatalf("server address %q is not a valid IP address", *serverAddress)
		}
	}

	ctx := context.Background()

	exporter, err := otlp.NewExporter(
		otlp.WithInsecure(),
		otlp.WithAddress("api-dogfood.honeycomb.io:9090"),
		otlp.WithHeaders(map[string]string{
			"x-honeycomb-apikey": *apikey,
			"dataset":            *dataset,
		}),
	)

	if err != nil {
		log.Fatal(err)
	}
	defer exporter.Shutdown(context.Background())
	defer initTracer(exporter)(context.Background())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go handleSignals(cancel)
	stop := ctx.Done()

	handler := otelhttp.NewHandler(makeHandler(), "serve-http",
		otelhttp.WithPublicEndpoint(),
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents))
	if err := runHTTPServer(serverIPAddress, *serverPort, handler, stop); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func speakPlainTextTo(w http.ResponseWriter) {
	w.Header().Add("Content-Type", "text/plain")
}

func makeHandler() http.Handler {
	userNameKey := label.Key("username")
	var mux http.ServeMux
	mux.Handle("/hello",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			span := trace.SpanFromContext(ctx)
			span.SetAttributes(label.String("ex.com/another", "yes"))

			eventAttrs := make([]label.KeyValue, 1, 2)
			eventAttrs[0] = label.Int("request-handled", 100)
			userNameVal := baggage.Value(ctx, label.Key("username"))
			if userNameVal.Type() != label.INVALID {
				attr := label.KeyValue{
					Key:   userNameKey,
					Value: userNameVal,
				}
				span.SetAttributes(attr)
				eventAttrs = append(eventAttrs, attr)
			}
			span.AddEvent("handling this...", trace.WithAttributes(eventAttrs...))

			speakPlainTextTo(w)
			_, err := io.WriteString(w, "Hello, world!\n")
			span.RecordError(err)
		}))
	return &mux
}
