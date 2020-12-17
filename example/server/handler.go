package main

import (
	"io"
	"net/http"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

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
