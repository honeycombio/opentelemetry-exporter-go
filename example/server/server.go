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
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/propagators"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer(exporter *honeycomb.Exporter) func() {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
	tp.ApplyConfig(
		// The default takes parent sampler hints into account, which we don't need here.
		sdktrace.Config{
			// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
			// In a production application, use sdktrace.ProbabilitySampler with a desired
			// probability.
			DefaultSampler: sdktrace.AlwaysSample(),
		})
	global.SetTracerProvider(tp)
	global.SetTextMapPropagator(otel.NewCompositeTextMapPropagator(
		propagators.TraceContext{},
		propagators.Baggage{}))
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
	defer initTracer(exporter)()

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
