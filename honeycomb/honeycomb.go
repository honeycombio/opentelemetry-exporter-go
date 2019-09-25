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
	"fmt"
	"github.com/google/uuid"
	"go.opentelemetry.io/api/core"
	"google.golang.org/grpc/codes"
	"time"

	libhoney "github.com/honeycombio/libhoney-go"
	"go.opentelemetry.io/sdk/trace"
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
}

// SpanEvent represents an event attached to a specific span.
type SpanEvent struct {
	Name          string  `json:"name"`
	TraceID       string  `json:"trace.trace_id"`
	ParentID      string  `json:"trace.parent_id,omitempty"`
	DurationMilli float64 `json:"duration_ms"`
	SpanEventType string  `json:"meta.span_type"`
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

func getHoneycombTraceID(traceIDHigh uint64, traceIDLow uint64) string {
	hcTraceUUID, _ := uuid.Parse(fmt.Sprintf("%016x%016x", traceIDHigh, traceIDLow))
	// TODO: what should we do with that error?

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
func NewExporter(config Config) *Exporter {
	// Developer note: bump this with each release
	versionStr := "0.0.1"
	libhoney.UserAgentAddition = "Honeycomb-OpenTelemetry-exporter/" + versionStr

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
	libhoney.Init(libhoneyConfig)
	builder := libhoney.NewBuilder()

	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	return &Exporter{
		Builder:     builder,
		ServiceName: config.ServiceName,
	}
}

func (e *Exporter) Register() {
	trace.Register()
	trace.RegisterExporter(e)
}

// ExportSpan exports a SpanData to Honeycomb.
func (e *Exporter) ExportSpan(data *trace.SpanData) {
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
			spanEv.AddField(kv.Key.Name, kv.Value.Emit())
		}
		spanEv.Timestamp = a.Time

		spanEv.Add(SpanEvent{
			Name:          a.Message,
			TraceID:       getHoneycombTraceID(data.SpanContext.TraceID.High, data.SpanContext.TraceID.Low),
			ParentID:      fmt.Sprintf("%d", data.SpanContext.SpanID),
			DurationMilli: 0,
			SpanEventType: "span_event",
		})
		spanEv.SendPresampled()
	}
	for name, value := range data.Attributes {
		// TODO: What will libhoney do if value is nil?
		ev.AddField(getValueFromAttribute(name, value))
	}

	ev.AddField("status.code", int32(data.Status))
	// If the status isn't zero, set error to be true
	if data.Status != 0 {
		ev.AddField("error", true)
	}

	ev.SendPresampled()
}

var _ trace.Exporter = (*Exporter)(nil)

func honeycombSpan(s *trace.SpanData) *Span {
	sc := s.SpanContext
	hcTraceID := getHoneycombTraceID(sc.TraceID.High, sc.TraceID.Low)
	hcSpan := &Span{
		TraceID:         hcTraceID,
		ID:              fmt.Sprintf("%d", sc.SpanID),
		Name:            s.Name,
		HasRemoteParent: s.HasRemoteParent,
	}

	if s.ParentSpanID != sc.SpanID && s.ParentSpanID != 0 {
		hcSpan.ParentID = fmt.Sprintf("%d", s.ParentSpanID)
	}

	if s, e := s.StartTime, s.EndTime; !s.IsZero() && !e.IsZero() {
		hcSpan.DurationMilli = float64(e.Sub(s)) / float64(time.Millisecond)
	}

	if s.Status != codes.OK {
		hcSpan.Error = true
	}

	// TODO: (akvanhar) add links
	// TODO: (akvanhar) add SpanRef type

	//var refs []*SpanRef
	//for _, link := range data.Links {
	//	refs = append(refs, &gen.SpanRef{
	//		TraceIdHigh: bytesToInt64(link.TraceID[0:8]),
	//		TraceIdLow:  bytesToInt64(link.TraceID[8:16]),
	//		SpanId:      bytesToInt64(link.SpanID[:]),
	//	})
	//}
	return hcSpan
}

func getValueFromAttribute(key string, value interface{}) (string, interface{}) {
	var tagValue interface{}
	switch value := value.(type) {
	case core.Value:
		fmt.Println("core.value")
		switch value.Type {
		case core.BOOL:
			tagValue = value.Bool
		case core.INT64:
			tagValue = value.Int64
		case core.UINT64:
			tagValue = value.Uint64
		case core.FLOAT64:
			tagValue = value.Float64
		case core.STRING:
			tagValue = value.String
		case core.BYTES:
			tagValue = value.Bytes
		}
	default:
		tagValue = value
	}
	return key, tagValue
}
