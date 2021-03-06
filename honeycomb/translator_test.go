package honeycomb

import (
	"math"
	"testing"
	"time"

	"go.opentelemetry.io/otel/label"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.opentelemetry.io/otel/codes"
	expTrace "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	apitrace "go.opentelemetry.io/otel/trace"
)

func TestOCProtoSpanToOTelSpanSnapshot(t *testing.T) {
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
		Status:                  &tracepb.Status{Code: int32(codes.Unset), Message: "status message"},
		SameProcessAsParentSpan: &wrappers.BoolValue{Value: false},
		ChildSpanCount:          &wrappers.UInt32Value{Value: 5},
		Resource: &resourcepb.Resource{
			Type: "host",
			Labels: map[string]string{
				"host.name": "xanadu",
			},
		},
	}

	want := &expTrace.SpanSnapshot{
		SpanContext:  spanContext([]byte{0x02}, []byte{0x03}),
		ParentSpanID: apitrace.SpanID{0x01},
		SpanKind:     apitrace.SpanKindClient,
		Name:         "trace-name",
		StartTime:    time.Unix(start.Unix(), int64(start.Nanosecond())),
		EndTime:      time.Unix(end.Unix(), int64(end.Nanosecond())),
		Attributes: []label.KeyValue{
			label.String("some-string", "some-value"),
			label.Float64("some-double", math.Pi),
			label.Int("some-int", 42),
			label.Bool("some-boolean", true),
		},
		Links: []apitrace.Link{
			{
				SpanContext: spanContext([]byte{0x04}, []byte{0x05}),
				Attributes: []label.KeyValue{
					label.Float64("e", math.E),
				},
			},
		},
		MessageEvents: []expTrace.Event{
			{
				Name: "test-event",
				Time: time.Unix(annotationTime.Unix(), int64(annotationTime.Nanosecond())),
				Attributes: []label.KeyValue{
					label.String("annotation-attr", "annotation-val"),
				},
			},
		},
		StatusCode:       codes.Unset,
		StatusMessage:    "status message",
		HasRemoteParent:  true,
		DroppedLinkCount: 2,
		ChildSpanCount:   5,
		Resource:         resource.NewWithAttributes(label.String("host.name", "xanadu")),
	}

	got, err := OCProtoSpanToOTelSpanSnapshot(&span)
	if err != nil {
		t.Fatalf("failed to convert proto span to otel span data: %v", err)
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(label.Value{}), cmp.AllowUnexported(trace.TraceState{}), cmpopts.SortSlices(keyValueLess)); diff != "" {
		t.Errorf("otel span: (-want +got):\n%s", diff)
	}
}

func keyValueLess(lhs, rhs label.KeyValue) bool {
	return lhs.Key < rhs.Key
}
