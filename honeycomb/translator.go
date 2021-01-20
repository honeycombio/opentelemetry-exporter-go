package honeycomb

import (
	"errors"
	"time"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	apitrace "go.opentelemetry.io/otel/trace"
)

// timestampToTime creates a Go time.Time value from a Google protobuf Timestamp.
func timestampToTime(ts *timestamp.Timestamp) (t time.Time) {
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
func spanContext(traceID []byte, spanID []byte) apitrace.SpanContext {
	ctx := apitrace.SpanContext{}
	if traceID != nil {
		copy(ctx.TraceID[:], traceID[:])
	}
	if spanID != nil {
		copy(ctx.SpanID[:], spanID[:])
	}
	return ctx
}

func spanResource(span *tracepb.Span) *resource.Resource {
	if span.Resource == nil {
		return nil
	}
	attrs := make([]label.KeyValue, len(span.Resource.Labels))
	i := 0
	for k, v := range span.Resource.Labels {
		attrs[i] = label.String(k, v)
		i++
	}
	return resource.NewWithAttributes(attrs...)
}

// Create []kv.KeyValue attributes from an OC *Span_Attributes
func createOTelAttributes(attributes *tracepb.Span_Attributes) []label.KeyValue {
	if attributes == nil || attributes.AttributeMap == nil {
		return nil
	}

	oTelAttrs := make([]label.KeyValue, len(attributes.AttributeMap))

	i := 0
	for key, attributeValue := range attributes.AttributeMap {
		keyValue := label.KeyValue{
			Key: label.Key(key),
		}
		switch val := attributeValue.Value.(type) {
		case *tracepb.AttributeValue_StringValue:
			keyValue.Value = label.StringValue(attributeValueAsString(attributeValue))
		case *tracepb.AttributeValue_BoolValue:
			keyValue.Value = label.BoolValue(val.BoolValue)
		case *tracepb.AttributeValue_IntValue:
			keyValue.Value = label.Int64Value(val.IntValue)
		case *tracepb.AttributeValue_DoubleValue:
			keyValue.Value = label.Float64Value(val.DoubleValue)
		}
		oTelAttrs[i] = keyValue
		i++
	}

	return oTelAttrs
}

// Create Span Links (including their attributes) from an OC Span
func createSpanLinks(spanLinks *tracepb.Span_Links) []apitrace.Link {
	if spanLinks == nil {
		return nil
	}

	links := make([]apitrace.Link, len(spanLinks.Link))

	for i, link := range spanLinks.Link {
		traceLink := apitrace.Link{
			SpanContext: spanContext(link.GetTraceId(), link.GetSpanId()),
			Attributes:  createOTelAttributes(link.Attributes),
		}
		links[i] = traceLink
	}

	return links
}

func createMessageEvents(spanEvents *tracepb.Span_TimeEvents) []trace.Event {
	if spanEvents == nil {
		return nil
	}

	annotations := 0
	for _, event := range spanEvents.TimeEvent {
		if annotation := event.GetAnnotation(); annotation != nil {
			annotations++
		}
	}

	events := make([]trace.Event, annotations)

	for i, event := range spanEvents.TimeEvent {
		if annotation := event.GetAnnotation(); annotation != nil {
			events[i] = trace.Event{
				Time:       timestampToTime(event.GetTime()),
				Name:       annotation.GetDescription().GetValue(),
				Attributes: createOTelAttributes(annotation.GetAttributes()),
			}
		}
	}

	return events
}

func attributeValueAsString(val *tracepb.AttributeValue) string {
	if wrapper := val.GetStringValue(); wrapper != nil {
		return wrapper.GetValue()
	}

	return ""
}

func getDroppedLinkCount(links *tracepb.Span_Links) int {
	if links != nil {
		return int(links.DroppedLinksCount)
	}

	return 0
}

func getChildSpanCount(span *tracepb.Span) int {
	if count := span.GetChildSpanCount(); count != nil {
		return int(count.GetValue())
	}

	return 0
}

func getSpanName(span *tracepb.Span) string {
	if name := span.GetName(); name != nil {
		return name.GetValue()
	}

	return ""
}

func getHasRemoteParent(span *tracepb.Span) bool {
	if sameProcess := span.GetSameProcessAsParentSpan(); sameProcess != nil {
		return !sameProcess.Value
	}

	return false
}

func getStatusCode(span *tracepb.Span) codes.Code {
	if span.Status != nil {
		return codes.Code(span.Status.Code)
	}

	return codes.Ok
}

func getStatusMessage(span *tracepb.Span) string {
	switch {
	case span.Status == nil:
		return codes.Ok.String()
	case span.Status.Message != "":
		return span.Status.Message
	default:
		return codes.Code(span.Status.Code).String()
	}
}

// OCProtoSpanToOTelSpanSnapshot converts an OC Span to an OTel SpanSnapshot
func OCProtoSpanToOTelSpanSnapshot(span *tracepb.Span) (*trace.SpanSnapshot, error) {
	if span == nil {
		return nil, errors.New("expected a non-nil span")
	}

	spanData := &trace.SpanSnapshot{
		SpanContext: spanContext(span.GetTraceId(), span.GetSpanId()),
	}

	copy(spanData.ParentSpanID[:], span.GetParentSpanId()[:])
	spanData.Name = getSpanName(span)
	spanData.SpanKind = oTelSpanKind(span.GetKind())
	spanData.Links = createSpanLinks(span.GetLinks())
	spanData.Attributes = createOTelAttributes(span.GetAttributes())
	spanData.MessageEvents = createMessageEvents(span.GetTimeEvents())
	spanData.StartTime = timestampToTime(span.GetStartTime())
	spanData.EndTime = timestampToTime(span.GetEndTime())
	spanData.StatusCode = getStatusCode(span)
	spanData.StatusMessage = getStatusMessage(span)
	spanData.HasRemoteParent = getHasRemoteParent(span)
	spanData.DroppedLinkCount = getDroppedLinkCount(span.GetLinks())
	spanData.ChildSpanCount = getChildSpanCount(span)
	spanData.Resource = spanResource(span)

	return spanData, nil
}
