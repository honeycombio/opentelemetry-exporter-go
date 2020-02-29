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

func TimestampToTime(ts *timestamp.Timestamp) (t time.Time) {
	if ts == nil {
		return
	}
	return time.Unix(ts.Seconds, int64(ts.Nanos))
}


func copySpanKind(sd *trace.SpanData, kind tracepb.Span_SpanKind) {
	// note that tracepb.SpanKindInternal, tracepb.SpanKindProducer and tracepb.SpanKindConsumer
	// have no equivalent OC proto type.
	switch kind {
		case tracepb.Span_SPAN_KIND_UNSPECIFIED:
			sd.SpanKind = apitrace.SpanKindUnspecified
		case tracepb.Span_SERVER:
			sd.SpanKind = apitrace.SpanKindServer
		case tracepb.Span_CLIENT:
			sd.SpanKind = apitrace.SpanKindClient
		default:
			sd.SpanKind = apitrace.SpanKindUnspecified
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

func copySpanAttributes(fromSpan *tracepb.Span, toSpan *trace.SpanData) {
	if fromSpan.Attributes != nil {
		if fromSpan.Attributes.AttributeMap != nil {
			toSpan.Attributes = make([]core.KeyValue, len(fromSpan.Attributes.AttributeMap))
			for key, value := range fromSpan.Attributes.AttributeMap {
				keyValue := core.KeyValue{
					Key: core.Key(key),
					// TODO(posman): handle non-string values
					Value: core.String(value.GetStringValue().GetValue()),
				}
				toSpan.Attributes = append(toSpan.Attributes, keyValue)
			}
		}
	}
}

func copySpanLinks(fromSpan *tracepb.Span, toSpan *trace.SpanData) {
	if fromSpan.Links == nil {
		return
	}

	toSpan.Links = make([]apitrace.Link, len(fromSpan.Links.Link))

	for _, link := range fromSpan.Links.Link {
		traceLink := apitrace.Link{
			SpanContext: spanContext(link.TraceId, link.SpanId),
		}

		if link.Attributes != nil {
			if link.Attributes.AttributeMap != nil {
				traceLink.Attributes = make([]core.KeyValue, len(link.Attributes.AttributeMap))
				for key, value := range link.Attributes.AttributeMap {
					keyValue := core.KeyValue{
						Key: core.Key(key),
						// TODO(posman): handle non-string values
						Value: core.String(value.GetStringValue().GetValue()),
					}
					traceLink.Attributes = append(traceLink.Attributes, keyValue)
				}
			}
		}

		toSpan.Links = append(toSpan.Links, traceLink)
	}

	toSpan.DroppedLinkCount = int(fromSpan.Links.DroppedLinksCount)
}

func ProtoSpanToOTelSpanData(span *tracepb.Span) (*trace.SpanData, error) {
	if span == nil {
		return nil, errors.New("expected a non-nil span")
	}

	spanData := &trace.SpanData{
		SpanContext: spanContext(span.TraceId, span.SpanId),
	}

	// Copy ParentSpanID, Span Kind and ChildSpanCount
	copy(spanData.ParentSpanID[:], span.ParentSpanId[:])
	copySpanKind(spanData, span.Kind)
	spanData.ChildSpanCount = int(span.ChildSpanCount.GetValue())
	copySpanLinks(span, spanData)
	copySpanAttributes(span, spanData)

	spanData.StartTime = TimestampToTime(span.StartTime)
	spanData.EndTime = TimestampToTime(span.EndTime)

	// TODO(posman)
	//	MessageEvents []Event = span.TimeEvents *Span_TimeEvents ????
	//	Status codes.Code = span.Status *Status
	//	HasRemoteParent bool = ! span.SameProcessAsParentSpan *wrappers.BoolValue
	//	DroppedAttributeCount int = ???
	//	DroppedMessageEventCount int = ???

	return spanData, nil
}


