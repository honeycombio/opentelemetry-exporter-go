// Copyright 2019, Honeycomb, Hound Technology, Inc.
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

// Package honeycomb contains a trace exporter for Honeycomb
package honeycomb

import (
	"context"
	"encoding/hex"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"

	libhoney "github.com/honeycombio/libhoney-go"
	"go.opentelemetry.io/otel/sdk/export/trace"
)

const (
	defaultApiKey  = "apikey-placeholder"
	defaultDataset = "opentelemetry"
)

type Config struct {
	// ApiKey is your Honeycomb authentication token, available from
	// https://ui.honeycomb.io/account. default: apikey-placeholder
	ApiKey string
	// Dataset is the name of the Honeycomb dataset to which events will be
	// sent. default: beeline-go
	Dataset string
	// Service Name identifies your application. While optional, setting this
	// field is extremely valuable when you instrument multiple services. If set
	// it will be added to all events as `service_name`
	ServiceName string
	// Debug will emit verbose logging to STDOUT when true. If you're having
	// trouble getting the beeline to work, set this to true in a dev
	// environment.
	Debug bool
	// APIHost is the hostname for the Honeycomb API server to which to send
	// these events. default: https://api.honeycomb.io/
	APIHost string
	// UserAgent will set the user agent used by the exporter
	UserAgent string
	// OnError is the hook to be called when there is
	// an error occurred when uploading the span data.
	// If no custom hook is set, errors are logged.
	OnError func(err error)
}

// Exporter is an implementation of trace.Exporter that uploads a span to Honeycomb
type Exporter struct {
	Builder        *libhoney.Builder
	SampleFraction float64
	// Service Name identifies your application. While optional, setting this
	// field is extremely valuable when you instrument multiple services. If set
	// it will be added to all events as `service_name`
	ServiceName string
	// Debug will emit verbose logging to STDOUT when true.
	// If you're having trouble getting the exporter to work, set this to true in a dev
	// environment
	Debug bool
	// OnError is the hook to be called when there is
	// an error occurred when uploading the span data.
	// If no custom hook is set, errors are logged.
	OnError func(err error)
}

// SpanEvent represents an event attached to a specific span.
type SpanEvent struct {
	Name     string `json:"name"`
	TraceID  string `json:"trace.trace_id"`
	ParentID string `json:"trace.parent_id,omitempty"`
	SpanType string `json:"meta.span_type"`
}

type SpanRefType int64

const (
	SpanRefType_CHILD_OF     SpanRefType = 0
	SpanRefType_FOLLOWS_FROM SpanRefType = 1
)

// Link represents a link to a trace and span that lives elsewhere.
// TraceID and ParentID are used to identify the span with which the trace is associated
// We are modeling Links for now as child spans rather than properties of the event.
type Link struct {
	TraceID     string      `json:"trace.trace_id"`
	ParentID    string      `json:"trace.parent_id,omitempty"`
	LinkTraceID string      `json:"trace.link.trace_id"`
	LinkSpanID  string      `json:"trace.link.span_id"`
	SpanType    string      `json:"meta.span_type"`
	RefType     SpanRefType `json:"ref_type,omitempty"`
}

// Span is the format of trace events that Honeycomb accepts
type Span struct {
	TraceID         string  `json:"trace.trace_id"`
	Name            string  `json:"name"`
	ID              string  `json:"trace.span_id"`
	ParentID        string  `json:"trace.parent_id,omitempty"`
	DurationMilli   float64 `json:"duration_ms"`
	Status          string  `json:"response.status_code,omitempty"`
	Error           bool    `json:"error,omitempty"`
	HasRemoteParent bool    `json:"has_remote_parent"`
}

func getHoneycombTraceID(traceID string) string {
	hcTraceUUID, err := uuid.Parse(traceID)
	if err != nil {
		return ""
	}
	return hcTraceUUID.String()
}

// Close waits for all in-flight messages to be sent. You should
// call Close() before app termination.
func (e *Exporter) Close() {
	libhoney.Close()
}

// NewExporter returns an implementation of trace.Exporter that uploads spans to Honeycomb
//
// apiKey is your Honeycomb apiKey (also known as your write key)
// dataset is the name of your Honeycomb dataset
//
// Don't have a Honeycomb account? Sign up at https://ui.honeycomb.io/signup
func NewExporter(config Config) (*Exporter, error) {
	// Developer note: bump this with each release
	versionStr := "0.2.1"

	if config.UserAgent != "" {
		libhoney.UserAgentAddition = config.UserAgent + "/" + versionStr
	} else {
		libhoney.UserAgentAddition = "Honeycomb-OpenTelemetry-exporter/" + versionStr
	}

	if config.ApiKey == "" {
		config.ApiKey = defaultApiKey
	}
	if config.Dataset == "" {
		config.Dataset = defaultDataset
	}

	libhoneyConfig := libhoney.Config{
		WriteKey: config.ApiKey,
		Dataset:  config.Dataset,
	}
	if config.Debug {
		libhoneyConfig.Logger = &libhoney.DefaultLogger{}
	}
	if config.APIHost != "" {
		libhoneyConfig.APIHost = config.APIHost
	}

	err := libhoney.Init(libhoneyConfig)
	if err != nil {
		return nil, err
	}
	builder := libhoney.NewBuilder()

	onError := func(err error) {
		if config.OnError != nil {
			config.OnError(err)
			return
		}
		log.Printf("Error when sending spans to Honeycomb: %v", err)
	}

	return &Exporter{
		Builder:     builder,
		ServiceName: config.ServiceName,
		OnError:     onError,
	}, nil
}

// ExportSpan exports a SpanData to Honeycomb.
func (e *Exporter) ExportSpan(ctx context.Context, data *trace.SpanData) {
	ev := e.Builder.NewEvent()

	if e.ServiceName != "" {
		ev.AddField("service_name", e.ServiceName)
	}

	ev.Timestamp = data.StartTime
	hs := honeycombSpan(data)
	ev.Add(hs)

	// We send these message events as 0 duration spans
	for _, a := range data.MessageEvents {
		spanEv := e.Builder.NewEvent()
		if e.ServiceName != "" {
			spanEv.AddField("service_name", e.ServiceName)
		}

		for _, kv := range a.Attributes {
			spanEv.AddField(string(kv.Key), kv.Value.Emit())
		}
		spanEv.Timestamp = a.Time

		spanEv.Add(SpanEvent{
			Name:     a.Name,
			TraceID:  getHoneycombTraceID(data.SpanContext.TraceIDString()),
			ParentID: data.SpanContext.SpanIDString(),
			SpanType: "span_event",
		})
		err := spanEv.Send()
		if err != nil {
			e.OnError(err)
		}
	}

	for _, link := range data.Links {
		linkEv := e.Builder.NewEvent()
		linkEv.Add(Link{
			TraceID:     getHoneycombTraceID(data.SpanContext.TraceIDString()),
			ParentID:    data.SpanContext.SpanIDString(),
			LinkTraceID: getHoneycombTraceID(link.TraceIDString()),
			LinkSpanID:  link.SpanIDString(),
			SpanType:    "link",
			// TODO(akvanhar): properly set the reference type when specs are defined
			// see https://github.com/open-telemetry/opentelemetry-specification/issues/65
			RefType: SpanRefType_CHILD_OF,

			// TODO(akvanhar) add support for link.Attributes
		})
		err := linkEv.Send()
		if err != nil {
			e.OnError(err)
		}
	}

	for _, kv := range data.Attributes {
		ev.AddField(string(kv.Key), kv.Value.AsInterface())
	}

	ev.AddField("status.code", int32(data.Status))
	// If the status isn't zero, set error to be true
	if data.Status != 0 {
		ev.AddField("error", true)
	}

	err := ev.SendPresampled()
	if err != nil {
		e.OnError(err)
	}
}

var _ trace.SpanSyncer = (*Exporter)(nil)

func honeycombSpan(s *trace.SpanData) *Span {
	sc := s.SpanContext

	hcSpan := &Span{
		TraceID:         getHoneycombTraceID(sc.TraceIDString()),
		ID:              sc.SpanIDString(),
		Name:            s.Name,
		HasRemoteParent: s.HasRemoteParent,
	}
	parentID := hex.EncodeToString(s.ParentSpanID[:])
	var initializedParentID [8]byte
	if s.ParentSpanID != sc.SpanID && s.ParentSpanID != initializedParentID {
		hcSpan.ParentID = parentID
	}

	if s, e := s.StartTime, s.EndTime; !s.IsZero() && !e.IsZero() {
		hcSpan.DurationMilli = float64(e.Sub(s)) / float64(time.Millisecond)
	}

	if s.Status != codes.OK {
		hcSpan.Error = true
	}
	return hcSpan
}
