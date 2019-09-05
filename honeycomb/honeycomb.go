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
type Annotation struct {
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
}

// Span is the format of trace events that Honeycomb accepts
type Span struct {
	TraceID         string       `json:"trace.trace_id"`
	Name            string       `json:"name"`
	ID              uint64       `json:"trace.span_id"`
	ParentID        uint64       `json:"trace.parent_id,omitempty"`
	DurationMs      float64      `json:"duration_ms"`
	Timestamp       time.Time    `json:"timestamp,omitempty"`
	Annotations     []Annotation `json:"annotations,omitempty"`
	Status          string       `json:"response.status_code,omitempty"`
	Error           bool         `json:"error,omitempty"`
	HasRemoteParent bool         `json:"has_remote_parent"`
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
	versionStr := "1.0.1"
	libhoney.UserAgentAddition = "Honeycomb-OpenTelemetry-exporter/" + versionStr

	libhoney.Init(libhoney.Config{
		WriteKey: apiKey,
		Dataset:  dataset,
	})
	builder := libhoney.NewBuilder()
	// default sample reate is 1: aka no sampling.
	// set sampleRate on the exporter to be the sample rate given to the
	// ProbabilitySampler if used.
	return &Exporter{
		Builder:        builder,
		SampleFraction: 1,
		ServiceName:    "",
	}
}

// ExportSpan exports a SpanData to Jaeger.
func (e *Exporter) ExportSpan(data *trace.SpanData) {
	ev := e.Builder.NewEvent()
	if e.SampleFraction != 0 {
		ev.SampleRate = uint(1 / e.SampleFraction)
	}
	if e.ServiceName != "" {
		ev.AddField("service_name", e.ServiceName)
	}
	// ev.Timestamp = sd.StartTime
	hs := honeycombSpan(data)
	ev.Add(hs)

	// Add an event field for each attribute
	// Hrm. Seems like attributes are trying to tell us something about the type of span
	// ev.Add(data.Attributes)
	for _, a := range data.MessageEvents {
		ev.Add(a)
	}

	// Add an event field for status code and status message
	// Should we try to translate these status cods?
	if data.Status != 0 {
		ev.AddField("status_code", data.Status)
	}
	ev.SendPresampled()
}

var _ trace.Exporter = (*Exporter)(nil)

func honeycombSpan(s *trace.SpanData) *Span {
	sc := s.SpanContext
	hcTraceUUID, _ := uuid.Parse(fmt.Sprintf("%x%016x", sc.TraceID.High, sc.TraceID.Low))
	// TODO: what should we do with that error?

	hcTraceID := hcTraceUUID.String()

	hcSpan := &Span{
		TraceID:         hcTraceID,
		ID:              sc.SpanID,
		Name:            s.Name,
		Timestamp:       s.StartTime,
		HasRemoteParent: s.HasRemoteParent,
	}

	if s.ParentSpanID != (sc.SpanID) {
		hcSpan.ParentID = s.ParentSpanID
	}

	if s, e := s.StartTime, s.EndTime; !s.IsZero() && !e.IsZero() {
		hcSpan.DurationMs = float64(e.Sub(s)) / float64(time.Millisecond)
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
