package honeycomb

import (
	"errors"
	"time"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/api/core"
	apitrace "go.opentelemetry.io/otel/api/trace"
)

func attributeValueAsString(val *tracepb.AttributeValue) string {
	if wrapper := val.GetStringValue(); wrapper != nil {
		return wrapper.GetValue()
	}
	return ""
}

// Create a Golang time.Time from a Google protobuf Timestamp.
func TimestampToTime(ts *timestamp.Timestamp) (t time.Time) {
	if ts == nil {
		return
	}
	return time.Unix(ts.Seconds, int64(ts.Nanos))
}


// Get SpanKind from an OC Span_SpanKind
func oTelSpanKind(kind tracepb.Span_SpanKind) apitrace.SpanKind {
	// note that tracepb.SpanKindInternal, tracepb.SpanKindProducer and tracepb.SpanKindConsumer
	// have no equivalent OC proto type.
	switch kind {
		case tracepb.Span_SPAN_KIND_UNSPECIFIED:
			return apitrace.SpanKindUnspecified
		case tracepb.Span_SERVER:
			return apitrace.SpanKindServer
		case tracepb.Span_CLIENT:
			return apitrace.SpanKindClient
		default:
			return apitrace.SpanKindUnspecified
	}
}

// Creates an OpenTelemetry SpanContext from information in an OC Span.
// Note that the OC Span has no equivalent to TraceFlags field in the 
// OpenTelemetry SpanContext type.
func spanContext(traceId []byte, spanId []byte) core.SpanContext {
	ctx := core.SpanContext{}
	if traceId != nil {
		copy(ctx.TraceID[:], traceId[:])
	}
	if spanId != nil {
		copy(ctx.SpanID[:], spanId[:])
	}
	return ctx
}

// Create []core.KeyValue attributes from an OC *Span_Attributes
func createOTelAttributes(attributes *tracepb.Span_Attributes) []core.KeyValue {
	if attributes == nil || attributes.AttributeMap == nil {
		return nil
	}

	oTelAttrs := make([]core.KeyValue, len(attributes.AttributeMap))

	for key, attributeValue := range attributes.AttributeMap {
		keyValue := core.KeyValue{
			Key: core.Key(key),
		}
		switch value := attributeValue.Value.(type) {
		case *tracepb.AttributeValue_StringValue:
			keyValue.Value = core.String(attributeValueAsString(attributeValue))
		case *tracepb.AttributeValue_BoolValue:
			keyValue.Value = core.Bool(value.BoolValue)
		case *tracepb.AttributeValue_IntValue:
			keyValue.Value = core.Int64(value.IntValue)
		case *tracepb.AttributeValue_DoubleValue:
			keyValue.Value = core.Float64(value.DoubleValue)
		}
		oTelAttrs = append(oTelAttrs, keyValue)
	}

	return oTelAttrs
}

// Copy Span Links (including their attributes) from an OC Span to an OTel SpanData
func copySpanLinks(fromSpan *tracepb.Span, toSpan *trace.SpanData) {
	if fromSpan.Links == nil {
		return
	}

	toSpan.Links = make([]apitrace.Link, len(fromSpan.Links.Link))

	for _, link := range fromSpan.Links.Link {
		traceLink := apitrace.Link{
			SpanContext: spanContext(link.TraceId, link.SpanId),
		}

		traceLink.Attributes = createOTelAttributes(link.Attributes)

		toSpan.Links = append(toSpan.Links, traceLink)
	}

	toSpan.DroppedLinkCount = int(fromSpan.Links.DroppedLinksCount)
}

// Convert an OC Span to an OTel SpanData
func OCProtoSpanToOTelSpanData(span *tracepb.Span) (*trace.SpanData, error) {
	if span == nil {
		return nil, errors.New("expected a non-nil span")
	}

	spanData := &trace.SpanData{
		SpanContext: spanContext(span.GetTraceId(), span.GetSpanId()),
	}

	copy(spanData.ParentSpanID[:], span.GetParentSpanId()[:])
	spanData.SpanKind = oTelSpanKind(span.GetKind())
	spanData.ChildSpanCount = int(span.GetChildSpanCount().GetValue())
	copySpanLinks(span, spanData)
	spanData.Attributes = createOTelAttributes(span.GetAttributes())

	spanData.StartTime = TimestampToTime(span.GetStartTime())
	spanData.EndTime = TimestampToTime(span.GetEndTime())

	return spanData, nil
}


