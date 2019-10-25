package honeycomb

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/api/core"
	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/global"
	"go.opentelemetry.io/sdk/export"
	"google.golang.org/grpc/codes"

	libhoney "github.com/honeycombio/libhoney-go"
	apitrace "go.opentelemetry.io/api/trace"
	sdktrace "go.opentelemetry.io/sdk/trace"
)

func TestExport(t *testing.T) {
	now := time.Now().Round(time.Microsecond)
	traceID, _ := core.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID := uint64(0x0102030405060708)

	expectedTraceID := "01020304-0506-0708-090a-0b0c0d0e0f10"
	expectedSpanID := "72623859790382856"

	tests := []struct {
		name string
		data *export.SpanData
		want *Span
	}{
		{
			name: "no parent",
			data: &export.SpanData{
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
			data: &export.SpanData{
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
			data: &export.SpanData{
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
			data: &export.SpanData{
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

func setUpTestExporter(mockHoneycomb *libhoney.MockOutput) (apitrace.Tracer, error) {
	exporter, err := NewExporter(Config{
		ApiKey:      "overridden",
		Dataset:     "overridden",
		ServiceName: "opentelemetry-test",
	})

	libhoney.Init(libhoney.Config{
		WriteKey: "test",
		Dataset:  "test",
		Output:   mockHoneycomb,
	})
	exporter.Builder = libhoney.NewBuilder()

	exporter.RegisterSimpleSpanProcessor()
	tp, err := sdktrace.NewProvider(sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
	global.SetTraceProvider(tp)

	tr := global.TraceProvider().GetTracer("honeycomb/test")
	return tr, err
}

func TestHoneycombOutput(t *testing.T) {
	mockHoneycomb := &libhoney.MockOutput{}
	assert := assert.New(t)
	tr, err := setUpTestExporter(mockHoneycomb)
	assert.Equal(err, nil)

	_, span := tr.Start(context.TODO(), "myTestSpan")
	span.SetAttribute(key.New("ex.com/string").String("yes"))
	span.SetAttribute(key.New("ex.com/bool").Bool(true))
	span.SetAttribute(key.New("ex.com/int64").Int64(42))
	span.SetAttribute(key.New("ex.com/float64").Float64(3.14))
	var nilString string
	span.SetAttribute(key.New("ex.com/nil").String(nilString))
	time.Sleep(time.Duration(0.5 * float64(time.Millisecond)))

	span.End()

	assert.Equal(1, len(mockHoneycomb.Events()))
	mainEventFields := mockHoneycomb.Events()[0].Fields()
	traceID := mainEventFields["trace.trace_id"]
	honeycombTranslatedTraceUUID, _ := uuid.Parse(span.SpanContext().TraceIDString())
	honeycombTranslatedTraceID := honeycombTranslatedTraceUUID.String()

	assert.Equal(honeycombTranslatedTraceID, traceID)

	spanID := mainEventFields["trace.span_id"]
	expectedSpanID := fmt.Sprintf("%d", span.SpanContext().SpanID)
	assert.Equal(expectedSpanID, spanID)

	name := mainEventFields["name"]
	assert.Equal("honeycomb/test/myTestSpan", name)

	durationMilli := mainEventFields["duration_ms"]
	durationMilliFl, ok := durationMilli.(float64)
	assert.Equal(ok, true)
	assert.Equal((durationMilliFl > 0), true)
	assert.Equal((durationMilliFl < 1), true)

	serviceName := mainEventFields["service_name"]
	assert.Equal("opentelemetry-test", serviceName)
	assert.Equal(mockHoneycomb.Events()[0].Dataset, "test")

	attribute := mainEventFields["ex.com/string"]
	assert.Equal("yes", attribute)
	attribute = mainEventFields["ex.com/bool"]
	assert.Equal(true, attribute)
	attribute = mainEventFields["ex.com/int64"]
	assert.Equal(int64(42), attribute)
	attribute = mainEventFields["ex.com/float64"]
	assert.Equal(3.14, attribute)
	attribute = mainEventFields["ex.com/nil"]
	assert.Equal("", attribute)
}
func TestHoneycombOutputWithMessageEvent(t *testing.T) {
	mockHoneycomb := &libhoney.MockOutput{}
	assert := assert.New(t)
	tr, err := setUpTestExporter(mockHoneycomb)
	assert.Equal(err, nil)

	ctx, span := tr.Start(context.TODO(), "myTestSpan")
	span.AddEvent(ctx, "handling this...", key.New("request-handled").Int(100))
	time.Sleep(time.Duration(0.5 * float64(time.Millisecond)))

	span.End()

	assert.Equal(2, len(mockHoneycomb.Events()))

	// Check the fields on the main span event
	messageEventFields := mockHoneycomb.Events()[1].Fields()
	traceID := messageEventFields["trace.trace_id"]
	honeycombTranslatedTraceUUID, _ := uuid.Parse(span.SpanContext().TraceIDString())
	honeycombTranslatedTraceID := honeycombTranslatedTraceUUID.String()

	assert.Equal(honeycombTranslatedTraceID, traceID)

	spanID := messageEventFields["trace.span_id"]
	expectedSpanID := fmt.Sprintf("%d", span.SpanContext().SpanID)
	assert.Equal(expectedSpanID, spanID)

	name := messageEventFields["name"]
	assert.Equal("honeycomb/test/myTestSpan", name)

	durationMilli := messageEventFields["duration_ms"]
	durationMilliFl, ok := durationMilli.(float64)
	assert.Equal(ok, true)
	assert.Equal((durationMilliFl > 0), true)

	serviceName := messageEventFields["service_name"]
	assert.Equal("opentelemetry-test", serviceName)
	assert.Equal(mockHoneycomb.Events()[1].Dataset, "test")

	// Check the fields on the 0 duration Message Event
	mainEventFields := mockHoneycomb.Events()[0].Fields()
	msgEventName := mainEventFields["name"]
	assert.Equal("handling this...", msgEventName)

	attribute := mainEventFields["request-handled"]
	assert.Equal("100", attribute)

	msgEventTraceID := mainEventFields["trace.trace_id"]
	assert.Equal(honeycombTranslatedTraceID, msgEventTraceID)

	msgEventParentID := mainEventFields["trace.parent_id"]
	assert.Equal(spanID, msgEventParentID)

	msgEventServiceName := mainEventFields["service_name"]
	assert.Equal("opentelemetry-test", msgEventServiceName)

	spanEvent := mainEventFields["meta.span_type"]
	assert.Equal("span_event", spanEvent)
}
func TestHoneycombOutputWithLinks(t *testing.T) {
	linkTraceID, _ := core.TraceIDFromHex("0102030405060709090a0b0c0d0e0f11")
	linkSpanID := uint64(0x0102030405060709)

	mockHoneycomb := &libhoney.MockOutput{}
	assert := assert.New(t)

	tr, err := setUpTestExporter(mockHoneycomb)
	assert.Equal(err, nil)

	_, span := tr.Start(context.TODO(), "myTestSpan")
	span.AddLink(apitrace.Link{
		SpanContext: core.SpanContext{
			TraceID: linkTraceID,
			SpanID:  linkSpanID,
		},
	})

	span.End()

	assert.Equal(2, len(mockHoneycomb.Events()))

	// Check the fields on the main span event
	linkFields := mockHoneycomb.Events()[0].Fields()
	mainEventFields := mockHoneycomb.Events()[1].Fields()
	traceID := linkFields["trace.trace_id"]
	honeycombTranslatedTraceUUID, _ := uuid.Parse(span.SpanContext().TraceIDString())
	honeycombTranslatedTraceID := honeycombTranslatedTraceUUID.String()

	assert.Equal(honeycombTranslatedTraceID, traceID)

	linkParentID := linkFields["trace.parent_id"]
	assert.Equal(mainEventFields["trace.span_id"], linkParentID)
	hclinkTraceID := linkFields["trace.link.trace_id"]
	linkTraceIDString := hex.EncodeToString(linkTraceID[:])
	assert.Equal(getHoneycombTraceID(linkTraceIDString), hclinkTraceID)
	hclinkSpanID := linkFields["trace.link.span_id"]
	assert.Equal(fmt.Sprintf("%d", linkSpanID), hclinkSpanID)
	linkSpanType := linkFields["meta.span_type"]
	assert.Equal("link", linkSpanType)
}
