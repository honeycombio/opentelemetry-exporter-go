package honeycomb

import (
	"math"
	"testing"
	"time"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opentelemetry.io/otel/api/core"
	apitrace "go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/sdk/export/trace"
	"google.golang.org/grpc/codes"
)

func TestOCProtoSpanToOTelSpanData(t *testing.T) {
	start := time.Now()
	end := start.Add(10 * time.Millisecond)
	annotationTime := start.Add(3 * time.Millisecond)

	startTimestamp, err := ptypes.TimestampProto(start)
	if err != nil {
		t.Fatalf("failed to convert time to timestamp: %v", err)
	}
	endTimestamp, err := ptypes.TimestampProto(end)
	if err != nil {
		t.Fatalf("failed to convert time to timestamp: %v", err)
	}
	annotationTimestamp, err := ptypes.TimestampProto(annotationTime)
	if err != nil {
		t.Fatalf("failed to convert time to timestamp: %v", err)
	}

	span := tracepb.Span{
		TraceId:      []byte{0x02},
		SpanId:       []byte{0x03},
		ParentSpanId: []byte{0x01},
		Name:         &tracepb.TruncatableString{Value: "trace-name"},
		Kind:         tracepb.Span_CLIENT,
		StartTime:    startTimestamp,
		EndTime:      endTimestamp,
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: map[string]*tracepb.AttributeValue{
				"some-string": {
					Value: &tracepb.AttributeValue_StringValue{
						StringValue: &tracepb.TruncatableString{Value: "some-value"},
					},
				},
				"some-double": {
					Value: &tracepb.AttributeValue_DoubleValue{DoubleValue: math.Pi},
				},
				"some-int": {
					Value: &tracepb.AttributeValue_IntValue{IntValue: 42},
				},
				"some-boolean": {
					Value: &tracepb.AttributeValue_BoolValue{BoolValue: true},
				},
			},
		},
		Links: &tracepb.Span_Links{
			Link: []*tracepb.Span_Link{
				{
					TraceId: []byte{0x04},
					SpanId:  []byte{0x05},
					Attributes: &tracepb.Span_Attributes{
						AttributeMap: map[string]*tracepb.AttributeValue{
							"e": {
								Value: &tracepb.AttributeValue_DoubleValue{DoubleValue: math.E},
							},
						},
					},
				},
			},
			DroppedLinksCount: 2,
		},
		TimeEvents: &tracepb.Span_TimeEvents{
			TimeEvent: []*tracepb.Span_TimeEvent{
				{
					Time: annotationTimestamp,
					Value: &tracepb.Span_TimeEvent_Annotation_{
						Annotation: &tracepb.Span_TimeEvent_Annotation{
							Description: &tracepb.TruncatableString{Value: "test-event"},
							Attributes: &tracepb.Span_Attributes{
								AttributeMap: map[string]*tracepb.AttributeValue{
									"annotation-attr": {
										Value: &tracepb.AttributeValue_StringValue{
											StringValue: &tracepb.TruncatableString{Value: "annotation-val"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status:                  &tracepb.Status{Code: int32(codes.Unknown)},
		SameProcessAsParentSpan: &wrappers.BoolValue{Value: false},
		ChildSpanCount:          &wrappers.UInt32Value{Value: 5},
	}

	want := &trace.SpanData{
		SpanContext:  spanContext([]byte{0x02}, []byte{0x03}),
		ParentSpanID: core.SpanID{0x01},
		SpanKind:     apitrace.SpanKindClient,
		Name:         "trace-name",
		StartTime:    time.Unix(start.Unix(), int64(start.Nanosecond())),
		EndTime:      time.Unix(end.Unix(), int64(end.Nanosecond())),
		Attributes: []core.KeyValue{
			core.Key("some-string").String("some-value"),
			core.Key("some-double").Float64(math.Pi),
			core.Key("some-int").Int(42),
			core.Key("some-boolean").Bool(true),
		},
		Links: []apitrace.Link{
			{
				SpanContext: spanContext([]byte{0x04}, []byte{0x05}),
				Attributes: []core.KeyValue{
					core.Key("e").Float64(math.E),
				},
			},
		},
		MessageEvents: []trace.Event{
			{
				Name: "test-event",
				Time: time.Unix(annotationTime.Unix(), int64(annotationTime.Nanosecond())),
				Attributes: []core.KeyValue{
					core.Key("annotation-attr").String("annotation-val"),
				},
			},
		},
		StatusCode:       codes.Unknown,
		HasRemoteParent:  true,
		DroppedLinkCount: 2,
		ChildSpanCount:   5,
	}

	got, err := OCProtoSpanToOTelSpanData(&span)
	if err != nil {
		t.Fatalf("failed to convert proto span to otel span data: %v", err)
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(core.Value{}), cmpopts.SortSlices(keyValueLess)); diff != "" {
		t.Errorf("otel span: (-want +got):\n%s", diff)
	}
}

func keyValueLess(lhs, rhs core.KeyValue) bool {
	return lhs.Key < rhs.Key
}
