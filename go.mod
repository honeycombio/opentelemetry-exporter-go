module github.com/honeycombio/opentelemetry-exporter-go

go 1.12

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.3
	github.com/honeycombio/libhoney-go v1.12.4
	github.com/klauspost/compress v1.10.10 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/vmihailenco/msgpack/v4 v4.3.12 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.14.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.14.0
	go.opentelemetry.io/otel v0.14.0
	go.opentelemetry.io/otel/exporters/otlp v0.14.0
	go.opentelemetry.io/otel/sdk v0.14.0
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
)
