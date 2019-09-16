package honeycomb

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	libhoney "github.com/honeycombio/libhoney-go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/api/core"
	"go.opentelemetry.io/api/key"
	apitrace "go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/sdk/trace"
)

func TestExport(t *testing.T) {
	now := time.Now().Round(time.Microsecond)
	traceID := core.TraceID{High: 0x0102030405060708, Low: 0x090a0b0c0d0e0f10}
	spanID := uint64(0x0102030405060708)
	expectedTraceID := "01020304-0506-0708-090a-0b0c0d0e0f10"
	expectedSpanID := "72623859790382856"

	tests := []struct {
		name string
		data *trace.SpanData
		want *Span
	}{
		{
			name: "no parent",
			data: &trace.SpanData{
				SpanContext: core.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:      "/foo",
				StartTime: now,
				EndTime:   now,
			},
			want: &Span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/foo",
				DurationMilli: 0,
				Error:         false,
			},
		},
		{
			name: "1 day duration",
			data: &trace.SpanData{
				SpanContext: core.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:      "/bar",
				StartTime: now,
				EndTime:   now.Add(24 * time.Hour),
			},
			want: &Span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/bar",
				DurationMilli: 86400000,
				Error:         false,
			},
		},
		{
			name: "status code OK",
			data: &trace.SpanData{
				SpanContext: core.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:      "/baz",
				StartTime: now,
				EndTime:   now,
				Status:    codes.OK,
			},
			want: &Span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/baz",
				DurationMilli: 0,
				Error:         false,
			},
		},
		{
			name: "status code not OK",
			data: &trace.SpanData{
				SpanContext: core.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:      "/bazError",
				StartTime: now,
				EndTime:   now,
				Status:    codes.PermissionDenied,
			},
			want: &Span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/bazError",
				DurationMilli: 0,
				Error:         true,
			},
		},
	}
	for _, tt := range tests {
		got := honeycombSpan(tt.data)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("honeycombSpan:\n\tgot  %#v\n\twant %#v", got, tt.want)
		}
	}
}

func TestHoneycombOutput(t *testing.T) {
	mockHoneycomb := &libhoney.MockOutput{}
	assert := assert.New(t)

	trace.Register()
	exporter := NewExporter("overridden", "overridden")
	exporter.ServiceName = "opentelemetry-test"

	libhoney.Init(libhoney.Config{
		WriteKey: "test",
		Dataset:  "test",
		Output:   mockHoneycomb,
	})
	exporter.Builder = libhoney.NewBuilder()

	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	_, span := apitrace.GlobalTracer().Start(context.TODO(), "myTestSpan")
	time.Sleep(time.Duration(0.5 * float64(time.Millisecond)))

	span.Finish()

	assert.Equal(1, len(mockHoneycomb.Events()))
	traceID := mockHoneycomb.Events()[0].Fields()["trace.trace_id"]
	honeycombTranslatedTraceUUID, _ := uuid.Parse(fmt.Sprintf("%016x%016x", span.SpanContext().TraceID.High, span.SpanContext().TraceID.Low))
	honeycombTranslatedTraceID := honeycombTranslatedTraceUUID.String()

	assert.Equal(honeycombTranslatedTraceID, traceID)

	spanID := mockHoneycomb.Events()[0].Fields()["trace.span_id"]
	expectedSpanID := fmt.Sprintf("%d", span.SpanContext().SpanID)
	assert.Equal(expectedSpanID, spanID)

	name := mockHoneycomb.Events()[0].Fields()["name"]
	assert.Equal("myTestSpan", name)

	durationMilli := mockHoneycomb.Events()[0].Fields()["duration_ms"]
	durationMilliFl, ok := durationMilli.(float64)
	assert.Equal(ok, true)
	assert.Equal((durationMilliFl > 0), true)
	assert.Equal((durationMilliFl < 1), true)

	serviceName := mockHoneycomb.Events()[0].Fields()["service_name"]
	assert.Equal("opentelemetry-test", serviceName)
	assert.Equal(mockHoneycomb.Events()[0].Dataset, "test")
}
func TestHoneycombOutputWithMessageEvent(t *testing.T) {
	mockHoneycomb := &libhoney.MockOutput{}
	assert := assert.New(t)

	trace.Register()
	exporter := NewExporter("overridden", "overridden")
	exporter.ServiceName = "opentelemetry-test"

	libhoney.Init(libhoney.Config{
		WriteKey: "test",
		Dataset:  "test",
		Output:   mockHoneycomb,
	})
	exporter.Builder = libhoney.NewBuilder()

	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	ctx, span := apitrace.GlobalTracer().Start(context.TODO(), "myTestSpan")
	span.AddEvent(ctx, "handling this...", key.New("request-handled").Int(100))
	time.Sleep(time.Duration(0.5 * float64(time.Millisecond)))

	span.Finish()

	assert.Equal(2, len(mockHoneycomb.Events()))

	// Check the fields on the main span event
	traceID := mockHoneycomb.Events()[1].Fields()["trace.trace_id"]
	honeycombTranslatedTraceUUID, _ := uuid.Parse(fmt.Sprintf("%016x%016x", span.SpanContext().TraceID.High, span.SpanContext().TraceID.Low))
	honeycombTranslatedTraceID := honeycombTranslatedTraceUUID.String()

	assert.Equal(honeycombTranslatedTraceID, traceID)

	spanID := mockHoneycomb.Events()[1].Fields()["trace.span_id"]
	expectedSpanID := fmt.Sprintf("%d", span.SpanContext().SpanID)
	assert.Equal(expectedSpanID, spanID)

	name := mockHoneycomb.Events()[1].Fields()["name"]
	assert.Equal("myTestSpan", name)

	durationMilli := mockHoneycomb.Events()[1].Fields()["duration_ms"]
	durationMilliFl, ok := durationMilli.(float64)
	assert.Equal(ok, true)
	assert.Equal((durationMilliFl > 0), true)
	assert.Equal((durationMilliFl < 1), true)

	serviceName := mockHoneycomb.Events()[1].Fields()["service_name"]
	assert.Equal("opentelemetry-test", serviceName)
	assert.Equal(mockHoneycomb.Events()[1].Dataset, "test")

	// Check the fields on the 0 duration Message Event
	msgEventName := mockHoneycomb.Events()[0].Fields()["name"]
	assert.Equal("handling this...", msgEventName)

	attribute := mockHoneycomb.Events()[0].Fields()["request-handled"]
	assert.Equal("100", attribute)

	msgEventTraceID := mockHoneycomb.Events()[0].Fields()["trace.trace_id"]
	assert.Equal(honeycombTranslatedTraceID, msgEventTraceID)

	msgEventParentID := mockHoneycomb.Events()[0].Fields()["trace.parent_id"]
	assert.Equal(spanID, msgEventParentID)

	msgEventDurationMilli := mockHoneycomb.Events()[0].Fields()["duration_ms"]
	assert.Equal(float64(0), msgEventDurationMilli)

	msgEventServiceName := mockHoneycomb.Events()[0].Fields()["service_name"]
	assert.Equal("opentelemetry-test", msgEventServiceName)

	spanEvent := mockHoneycomb.Events()[0].Fields()["meta.span_event"]
	assert.Equal(true, spanEvent)
}
