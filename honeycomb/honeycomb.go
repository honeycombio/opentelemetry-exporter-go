// Copyright 2018, Honeycomb, Hound Technology, Inc.
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
	"github.com/davecgh/go-spew/spew"
	"google.golang.org/grpc/codes"
	"time"

	libhoney "github.com/honeycombio/libhoney-go"
	// apitrace "go.opentelemetry.io/api/trace"
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

// Attributes:
//  - Key
//  - VType
//  - VStr
//  - VDouble
//  - VBool
//  - VLong
//  - VBinary
type Tag struct {
	Key string `thrift:"key,1,required" db:"key" json:"key"`
	// VType   TagType  `json:"vType"`
	VStr    *string  `json:"vStr,omitempty"`
	VDouble *float64 `thrift:"vDouble,4" db:"vDouble" json:"vDouble,omitempty"`
	VBool   *bool    `thrift:"vBool,5" db:"vBool" json:"vBool,omitempty"`
	VLong   *int64   `thrift:"vLong,6" db:"vLong" json:"vLong,omitempty"`
	VBinary []byte   `thrift:"vBinary,7" db:"vBinary" json:"vBinary,omitempty"`
}

type Message struct {
	Timestamp int64  `json:"timestamp"`
	Fields    []*Tag `json:"fields"`
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
	// if len(sd.Attributes) != 0 {
	// 	for key, value := range sd.Attributes {
	// 		ev.AddField(key, value)
	// 	}
	// }

	// // Add an event field for status code and status message
	if data.Status != 0 {
		ev.AddField("status_code", data.Status)
	}
	// if sd.Status.Message != "" {
	// 	ev.AddField("status_description", sd.Status.Message)
	// }
	ev.SendPresampled()
}

var _ trace.Exporter = (*Exporter)(nil)

func honeycombSpan(s *trace.SpanData) *Span {
	// SpanData looks like:
	// SpanContext: (core.SpanContext) {
	//    TraceID: (core.TraceID) {
	// 	    High: (uint64) 4276129246450405016,
	// 	    Low: (uint64) 10285741614377517610
	//    },
	//    SpanID: (uint64) 306762295059959009,
	//    TraceOptions: (uint8) 1
	//   },
	//   ParentSpanID: (uint64) 0,
	//   SpanKind: (int) 0,
	//   Name: (string) (len=4) "/foo",
	//   StartTime: (time.Time) 2019-09-04 15:45:23.737295 -0700 PDT m=+0.001520965,
	//   EndTime: (time.Time) 2019-09-04 15:45:23.741027919 -0700 PDT m=+0.005253884,
	//   Attributes: (map[string]interface {}) <nil>,
	//   MessageEvents: ([]trace.event) <nil>,
	//   Status: (codes.Code) OK,
	//   HasRemoteParent: (bool) false,
	//   DroppedAttributeCount: (int) 0,
	//   DroppedMessageEventCount: (int) 0,
	//   DroppedLinkCount: (int) 0,
	//   ChildSpanCount: (int) 1
	sc := s.SpanContext
	hcTraceID := fmt.Sprintf("%x%016x", sc.TraceID.High, sc.TraceID.Low)
	spew.Dump(hcTraceID)
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

	// var messages []*Message
	// for _, a := range s.MessageEvents {
	// 	fields := make([]*gen.Tag, 0, len(a.Attributes()))
	// 	for _, kv := range a.Attributes() {
	// 		tag := attributeToTag(kv.Key.Variable.Name, kv.Value.Emit())
	// 		if tag != nil {
	// 			fields = append(fields, tag)
	// 		}
	// 	}
	// 	fields = append(fields, attributeToTag("message", a.Message()))
	// 	messages = append(messages, &Message{
	// 		//Timestamp: a.Time.UnixNano() / 1000,
	// 		//TODO: [rghetia] update when time is supported in the event.
	// 		Timestamp: time.Now().UnixNano() / 1000,
	// 		Fields:    fields,
	// 	})
	// }

	// if len(s.Annotations) != 0 || len(s.MessageEvents) != 0 {
	// 	hcSpan.Annotations = make([]Annotation, 0, len(s.Annotations)+len(s.MessageEvents))
	// 	for _, a := range s.Annotations {
	// 		hcSpan.Annotations = append(hcSpan.Annotations, Annotation{
	// 			Timestamp: a.Time,
	// 			Value:     a.Message,
	// 		})
	// 	}
	// 	for _, m := range s.MessageEvents {
	// 		a := Annotation{
	// 			Timestamp: m.Time,
	// 		}
	// 		switch m.EventType {
	// 		case trace.MessageEventTypeSent:
	// 			a.Value = "SENT"
	// 		case trace.MessageEventTypeRecv:
	// 			a.Value = "RECV"
	// 		default:
	// 			a.Value = "<?>"
	// 		}
	// 		hcSpan.Annotations = append(hcSpan.Annotations, a)
	// 	}
	// }
	return hcSpan
}
