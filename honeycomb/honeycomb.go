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
	"google.golang.org/grpc/codes"
	"time"

	libhoney "github.com/honeycombio/libhoney-go"
	"go.opentelemetry.io/sdk/trace"
)

// Exporter is an implementation of trace.Exporter that uploads a span to Honeycomb
type Exporter struct {
	Builder        *libhoney.Builder
	SampleFraction float64
	// Service Name identifies your application. While optional, setting this
	// field is extremely valuable when you instrument multiple services. If set
	// it will be added to all events as `service_name`
	ServiceName string
}

// Annotation represents an annotation with a value and a timestamp.
type SpanEvent struct {
	Name          string  `json:"name"`
	TraceID       string  `json:"trace.trace_id"`
	ParentID      string  `json:"trace.parent_id,omitempty"`
	DurationMilli float64 `json:"duration_ms"`
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
// writeKey is your Honeycomb writeKey (also known as your API key)
// dataset is the name of your Honeycomb dataset to send trace events to
//
// Don't have a Honeycomb account? Sign up at https://ui.honeycomb.io/signup
func NewExporter(apiKey, dataset string) *Exporter {
	// Developer note: bump this with each release
	versionStr := "0.0.1"
	libhoney.UserAgentAddition = "Honeycomb-OpenTelemetry-exporter/" + versionStr

	libhoney.Init(libhoney.Config{
		WriteKey: apiKey,
		Dataset:  dataset,
	})
	builder := libhoney.NewBuilder()
	// default sample reate is 1: aka no sampling.
	// set sampleRate on the exporter to be the sample rate given to the
	// ProbabilitySampler if used.
	// TODO (akvanhar): Figure out how OpenTelemetry handles sampling
	return &Exporter{
		Builder: builder,
		// SampleFraction: 1,
		ServiceName: "",
	}
}

// ExportSpan exports a SpanData to Honeycomb.
func (e *Exporter) ExportSpan(data *trace.SpanData) {
	ev := e.Builder.NewEvent()
	// if e.SampleFraction != 0 {
	// 	ev.SampleRate = uint(1 / e.SampleFraction)
	// }
	if e.ServiceName != "" {
		ev.AddField("service_name", e.ServiceName)
	}

	ev.Timestamp = data.StartTime
	hs := honeycombSpan(data)
	ev.Add(hs)

	// TODO: (akvanhar) do something about the attributes
	// ev.Add(data.Attributes)

	// We send these message events as 0 duration spans
	for _, a := range data.MessageEvents {
		spanEv := e.Builder.NewEvent()
		if e.ServiceName != "" {
			spanEv.AddField("service_name", e.ServiceName)
		}

		for _, kv := range a.Attributes {
			spanEv.AddField(kv.Key.Variable.Name, kv.Value.Emit())
		}
		spanEv.Timestamp = a.Time

		spanEv.Add(SpanEvent{
			Name:          a.Message,
			TraceID:       getHoneycombTraceID(data.SpanContext.TraceID.High, data.SpanContext.TraceID.Low),
			ParentID:      fmt.Sprintf("%d", data.SpanContext.SpanID),
			DurationMilli: 0,
		})
		spanEv.SendPresampled()
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
