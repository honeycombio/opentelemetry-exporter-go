package honeycomb

import (
	"context"
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	"github.com/honeycombio/libhoney-go/transmission"
	"github.com/stretchr/testify/assert"

	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/otel/api/global"
	apitrace "go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestGetHoneycombTraceID(t *testing.T) {
	tests := []struct {
		name    string
		traceID string
		want    string
	}{
		{
			name:    "64-bit traceID",
			traceID: "cbe4decd12429177",
			want:    "cbe4decd12429177",
		},
		{
			name:    "128-bit zero-padded traceID",
			traceID: "0000000000000000cbe4decd12429177",
			want:    "cbe4decd12429177",
		},
		{
			name:    "128-bit non-zero-padded traceID",
			traceID: "f23b42eac289a0fdcde48fcbe3ab1a32",
			want:    "f23b42eac289a0fdcde48fcbe3ab1a32",
		},
		{
			name:    "Non-hex traceID",
			traceID: "foobar1",
			want:    "666f6f62617231",
		},
		{
			name:    "Longer non-hex traceID",
			traceID: "foobarbaz",
			want:    "666f6f6261726261",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceID, err := hex.DecodeString(tt.traceID)
			if err != nil {
				traceID = []byte(tt.traceID)
			}
			got := getHoneycombTraceID(traceID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHoneycombTraceID:\n\tgot:  %#v\n\twant: %#v", got, tt.want)
			}
		})
	}
}

func TestExport(t *testing.T) {
	now := time.Now().Round(time.Microsecond)
	traceID, _ := apitrace.IDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := apitrace.SpanIDFromHex("0102030405060708")

	expectedTraceID := "0102030405060708090a0b0c0d0e0f10"
	expectedSpanID := "0102030405060708"

	tests := []struct {
		name string
		data *exporttrace.SpanData
		want *span
	}{
		{
			name: "no parent",
			data: &exporttrace.SpanData{
				SpanContext: apitrace.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:      "/foo",
				StartTime: now,
				EndTime:   now,
			},
			want: &span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/foo",
				DurationMilli: 0,
				Error:         false,
			},
		},
		{
			name: "1 day duration",
			data: &exporttrace.SpanData{
				SpanContext: apitrace.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:      "/bar",
				StartTime: now,
				EndTime:   now.Add(24 * time.Hour),
			},
			want: &span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/bar",
				DurationMilli: 86400000,
				Error:         false,
			},
		},
		{
			name: "status code OK",
			data: &exporttrace.SpanData{
				SpanContext: apitrace.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:       "/baz",
				StartTime:  now,
				EndTime:    now,
				StatusCode: codes.OK,
			},
			want: &span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/baz",
				DurationMilli: 0,
				Error:         false,
			},
		},
		{
			name: "status code not OK",
			data: &exporttrace.SpanData{
				SpanContext: apitrace.SpanContext{
					TraceID: traceID,
					SpanID:  spanID,
				},
				Name:       "/bazError",
				StartTime:  now,
				EndTime:    now,
				StatusCode: codes.PermissionDenied,
			},
			want: &span{
				TraceID:       expectedTraceID,
				ID:            expectedSpanID,
				Name:          "/bazError",
				DurationMilli: 0,
				Error:         true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := honeycombSpan(tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("honeycombSpan:\n\tgot  %#v\n\twant %#v", got, tt.want)
			}
		})
	}
}

func makeTestExporter(mockHoneycomb *transmission.MockSender, opts ...ExporterOption) (*Exporter, error) {
	return NewExporter(
		Config{
			APIKey: "overridden",
		},
		append(opts,
			TargetingDataset("test"),
			WithServiceName("opentelemetry-test"),
			withHoneycombSender(mockHoneycomb))...,
	)
}

func setUpTestProvider(exporter exporttrace.SpanSyncer, opts ...sdktrace.ProviderOption) (apitrace.Tracer, error) {
	tp, err := sdktrace.NewProvider(
		append([]sdktrace.ProviderOption{
			sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
			sdktrace.WithSyncer(exporter),
		}, opts...)...,
	)
	if err != nil {
		return nil, err
	}
	global.SetTraceProvider(tp)

	return global.TraceProvider().Tracer("honeycomb/test"), nil
}

func setUpTestExporter(mockHoneycomb *transmission.MockSender, opts ...ExporterOption) (apitrace.Tracer, error) {
	exporter, err := makeTestExporter(mockHoneycomb, opts...)
	if err != nil {
		return nil, err
	}
	return setUpTestProvider(exporter)
}

func TestHoneycombOutput(t *testing.T) {
	mockHoneycomb := &transmission.MockSender{}
	assert := assert.New(t)
	tr, err := setUpTestExporter(mockHoneycomb)
	assert.Nil(err)

	_, span := tr.Start(context.TODO(), "myTestSpan")
	var nilString string
	span.SetAttributes(
		label.String("ex.com/string", "yes"),
		label.Bool("ex.com/bool", true),
		label.Int64("ex.com/int64", 42),
		label.Float64("ex.com/float64", 3.14),
		label.String("ex.com/nil", nilString),
	)
	time.Sleep(time.Duration(0.5 * float64(time.Millisecond)))

	span.End()

	assert.Len(mockHoneycomb.Events(), 1)
	mainEventFields := mockHoneycomb.Events()[0].Data
	traceID := mainEventFields["trace.trace_id"]
	spanTraceID := span.SpanContext().TraceID
	honeycombTranslated := getHoneycombTraceID(spanTraceID[:])

	assert.Equal(honeycombTranslated, traceID)

	spanID := mainEventFields["trace.span_id"]
	expectedSpanID := span.SpanContext().SpanID.String()
	assert.Equal(expectedSpanID, spanID)

	name := mainEventFields["name"]
	assert.Equal("myTestSpan", name)

	durationMilli := mainEventFields["duration_ms"]
	durationMilliFl, ok := durationMilli.(float64)
	assert.True(ok)
	assert.Greater(durationMilliFl, 0.0)
	assert.Less(durationMilliFl, 5.0)

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
	mockHoneycomb := &transmission.MockSender{}
	assert := assert.New(t)
	tr, err := setUpTestExporter(mockHoneycomb)
	assert.Nil(err)

	ctx, span := tr.Start(context.TODO(), "myTestSpan")
	span.AddEvent(ctx, "handling this...", label.Int("request-handled", 100))
	time.Sleep(time.Duration(0.5 * float64(time.Millisecond)))

	span.End()

	assert.Len(mockHoneycomb.Events(), 2)

	// Check the fields on the main span event.
	mainEventFields := mockHoneycomb.Events()[1].Data
	traceID := mainEventFields["trace.trace_id"]
	spanTraceID := span.SpanContext().TraceID
	honeycombTranslatedTraceID := getHoneycombTraceID(spanTraceID[:])

	assert.Equal(honeycombTranslatedTraceID, traceID)

	spanID := mainEventFields["trace.span_id"]
	expectedSpanID := span.SpanContext().SpanID.String()
	assert.Equal(expectedSpanID, spanID)

	name := mainEventFields["name"]
	assert.Equal("myTestSpan", name)

	durationMilli := mainEventFields["duration_ms"]
	durationMilliFl, ok := durationMilli.(float64)
	assert.True(ok)
	assert.Greater(durationMilliFl, 0.0)

	serviceName := mainEventFields["service_name"]
	assert.Equal("opentelemetry-test", serviceName)
	assert.Equal(mockHoneycomb.Events()[1].Dataset, "test")

	// Check the fields on the zero-duration Message event.
	msgEventFields := mockHoneycomb.Events()[0].Data
	msgEventName := msgEventFields["name"]
	assert.Equal("handling this...", msgEventName)

	attribute := msgEventFields["request-handled"]
	assert.Equal(int64(100), attribute)

	msgEventTraceID := msgEventFields["trace.trace_id"]
	assert.Equal(honeycombTranslatedTraceID, msgEventTraceID)

	msgEventParentID := msgEventFields["trace.parent_id"]
	assert.Equal(spanID, msgEventParentID)

	msgEventParentName := msgEventFields["trace.parent_name"]
	assert.Equal("myTestSpan", msgEventParentName)

	msgEventServiceName := msgEventFields["service_name"]
	assert.Equal("opentelemetry-test", msgEventServiceName)

	spanEvent := msgEventFields["meta.annotation_type"]
	assert.Equal("span_event", spanEvent)
}

func TestHoneycombOutputWithLinks(t *testing.T) {
	linkTraceID, _ := apitrace.IDFromHex("0102030405060709090a0b0c0d0e0f11")
	linkSpanID, _ := apitrace.SpanIDFromHex("0102030405060709")

	mockHoneycomb := &transmission.MockSender{}
	assert := assert.New(t)

	tr, err := setUpTestExporter(mockHoneycomb)
	assert.Nil(err)

	_, span := tr.Start(context.TODO(), "myTestSpan", apitrace.LinkedTo(apitrace.SpanContext{
		TraceID: linkTraceID,
		SpanID:  linkSpanID,
	}))

	span.End()

	assert.Len(mockHoneycomb.Events(), 2)

	// Check the fields on the main span event.
	linkFields := mockHoneycomb.Events()[0].Data
	mainEventFields := mockHoneycomb.Events()[1].Data
	traceID := linkFields["trace.trace_id"]
	spanContextTraceID := span.SpanContext().TraceID
	honeycombTranslatedTraceID := getHoneycombTraceID(spanContextTraceID[:])

	assert.Equal(honeycombTranslatedTraceID, traceID)

	linkParentID := linkFields["trace.parent_id"]
	assert.Equal(mainEventFields["trace.span_id"], linkParentID)

	hclinkTraceID := linkFields["trace.link.trace_id"]
	assert.Equal(getHoneycombTraceID(linkTraceID[:]), hclinkTraceID)

	hclinkSpanID := linkFields["trace.link.span_id"]
	assert.Equal("0102030405060709", hclinkSpanID)
	linkAnnotationType := linkFields["meta.annotation_type"]
	assert.Equal("link", linkAnnotationType)
}

func TestHoneycombConfigValidation(t *testing.T) {
	tests := []struct {
		description string
		config      Config
		expectError bool
	}{
		{
			"empty API key",
			Config{},
			true,
		},
		{
			"populated API key",
			Config{
				APIKey: "xyz",
			},
			false,
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			assert := assert.New(t)
			exporter, err := NewExporter(test.config)
			if test.expectError {
				assert.Error(err)
				assert.Nil(exporter)
			} else {
				assert.Nil(err)
				assert.NotNil(exporter)
			}
		})
	}
}

func TestHoneycombStaticFieldValidation(t *testing.T) {
	tests := []struct {
		description string
		fieldName   string
		expectError bool
	}{
		{
			"empty name",
			"",
			true,
		},
		{
			"nonempty name",
			"xyz",
			false,
		},
	}
	config := Config{
		APIKey: "overridden",
	}
	var fieldValue interface{} = 1
	for _, test := range tests {
		for _, inMap := range []bool{false, true} {
			description := test.description
			if inMap {
				description += " (in map)"
			}
			t.Run(description, func(t *testing.T) {
				assert := assert.New(t)
				var opt ExporterOption
				if inMap {
					opt = WithFields(map[string]interface{}{
						test.fieldName: fieldValue,
					})
				} else {
					opt = WithField(test.fieldName, fieldValue)
				}
				exporter, err := NewExporter(config, opt)
				if test.expectError {
					assert.Error(err)
					assert.Nil(exporter)
				} else {
					assert.Nil(err)
					assert.NotNil(exporter)
				}
			})
		}
	}
}

func TestHoneycombDynamicFieldValidation(t *testing.T) {
	valueFunc := func() interface{} {
		return 1
	}
	tests := []struct {
		description string
		fieldName   string
		fieldValue  func() interface{}
		expectError bool
	}{
		{
			"empty name",
			"",
			valueFunc,
			true,
		},
		{
			"nil function",
			"xyz",
			nil,
			true,
		},
		{
			"nonempty name and function",
			"xyz",
			valueFunc,
			false,
		},
	}
	config := Config{
		APIKey: "overridden",
	}
	for _, test := range tests {
		for _, inMap := range []bool{false, true} {
			description := test.description
			if inMap {
				description += " (in map)"
			}
			t.Run(description, func(t *testing.T) {
				assert := assert.New(t)
				var opt ExporterOption
				if inMap {
					opt = WithDynamicFields(map[string]func() interface{}{
						test.fieldName: test.fieldValue,
					})
				} else {
					opt = WithDynamicField(test.fieldName, test.fieldValue)
				}
				exporter, err := NewExporter(config, opt)
				if test.expectError {
					assert.Error(err)
					assert.Nil(exporter)
				} else {
					assert.Nil(err)
					assert.NotNil(exporter)
				}
			})
		}
	}
}

func TestHoneycombOutputWithStaticFields(t *testing.T) {
	mockHoneycomb := &transmission.MockSender{}
	assert := assert.New(t)

	tr, err := setUpTestExporter(mockHoneycomb,
		WithField("a", 1),
		WithField("b", 2),
		WithFields(map[string]interface{}{
			"b": 4,
			"c": 5,
		}),
		WithField("a", 3))
	assert.Nil(err)

	_, span := tr.Start(context.TODO(), "myTestSpan")
	span.SetAttributes(
		label.String("ex.com/string", "yes"),
	)

	span.End()

	assert.Len(mockHoneycomb.Events(), 1)
	mainEventFields := mockHoneycomb.Events()[0].Data

	assert.Equal("yes", mainEventFields["ex.com/string"])
	assert.Equal(3, mainEventFields["a"])
	assert.Equal(4, mainEventFields["b"])
	assert.Equal(5, mainEventFields["c"])
}

func TestHoneycombOutputWithDynamicFields(t *testing.T) {
	mockHoneycomb := &transmission.MockSender{}
	assert := assert.New(t)

	constantly := func(v interface{}) func() interface{} {
		return func() interface{} {
			return v
		}
	}
	tr, err := setUpTestExporter(mockHoneycomb,
		WithDynamicField("a", constantly(1)),
		WithDynamicField("b", constantly(2)),
		WithDynamicFields(map[string]func() interface{}{
			"b": constantly(4),
			"c": constantly(5),
		}),
		WithDynamicField("a", constantly(3)))
	assert.Nil(err)

	_, span := tr.Start(context.TODO(), "myTestSpan")
	span.SetAttributes(
		label.String("ex.com/string", "yes"),
	)

	span.End()

	assert.Len(mockHoneycomb.Events(), 1)
	mainEventFields := mockHoneycomb.Events()[0].Data

	assert.Equal("yes", mainEventFields["ex.com/string"])
	assert.Equal(3, mainEventFields["a"])
	assert.Equal(4, mainEventFields["b"])
	assert.Equal(5, mainEventFields["c"])
}

func TestHoneycombOutputWithStaticAndDynamicFields(t *testing.T) {
	mockHoneycomb := &transmission.MockSender{}
	assert := assert.New(t)

	baseValue := 10
	delta := func(delta int) func() interface{} {
		return func() interface{} {
			return baseValue + delta
		}
	}
	tr, err := setUpTestExporter(mockHoneycomb,
		WithDynamicField("a", delta(1)),
		WithField("b", 2),
		WithDynamicFields(map[string]func() interface{}{
			// Replace a static field.
			"b": delta(4),
			"c": delta(5),
		}),
		// Replace a dynamic field.
		WithField("a", 3))
	assert.Nil(err)

	_, span := tr.Start(context.TODO(), "myTestSpan")
	span.SetAttributes(
		label.String("ex.com/string", "yes"),
	)

	span.End()

	assert.Len(mockHoneycomb.Events(), 1)
	mainEventFields := mockHoneycomb.Events()[0].Data

	assert.Equal("yes", mainEventFields["ex.com/string"])
	assert.Equal(3, mainEventFields["a"])
	assert.Equal(baseValue+4, mainEventFields["b"])
	assert.Equal(baseValue+5, mainEventFields["c"])
}

func TestHoneycombOutputWithResource(t *testing.T) {
	mockHoneycomb := &transmission.MockSender{}
	assert := assert.New(t)

	const (
		underlay int64 = iota
		middle
		overlay
	)

	exporter, err := makeTestExporter(mockHoneycomb,
		WithField("a", underlay),
		WithField("b", underlay))
	assert.Nil(err)
	assert.NotNil(exporter)

	tr, err := setUpTestProvider(exporter,
		sdktrace.WithResource(resource.New(
			label.Int64("a", middle),
			label.Int64("c", middle),
		)))

	ctx, span := tr.Start(context.TODO(), "myTestSpan")
	assert.Nil(err)
	span.SetAttributes(
		label.Int64("a", overlay),
		label.Int64("d", overlay),
	)
	span.AddEvent(ctx, "something", label.Int64("c", overlay))
	time.Sleep(time.Duration(0.5 * float64(time.Millisecond)))

	span.End()

	assert.Len(mockHoneycomb.Events(), 2)

	mainEventFields := mockHoneycomb.Events()[1].Data
	assert.Equal(int64(overlay), mainEventFields["a"])
	assert.Equal(int64(underlay), mainEventFields["b"])
	assert.Equal(int64(middle), mainEventFields["c"])
	assert.Equal(int64(overlay), mainEventFields["d"])

	messageEventFields := mockHoneycomb.Events()[0].Data
	assert.Equal(int64(middle), messageEventFields["a"])
	assert.Equal(int64(underlay), mainEventFields["b"])
	assert.Equal(int64(middle), mainEventFields["c"])
}
