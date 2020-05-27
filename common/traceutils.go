// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/opentracing/opentracing-go"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

type ContextMsg struct {
	Ctx context.Context
	Msg interface{}
}

var (
	// cs2sr is the latency of transporting
	cs2sr = mon.GetInstance().NewLabeledHistogram("cs_sr_latency_ms",
		[]string{"protocol", "topic", "annotation"}, nil)
	// sob2cs is the latency from business beginning to current module ending
	sob2cs = mon.GetInstance().NewLabeledHistogram("sob_cs_latency_ms",
		[]string{"protocol", "topic", "annotation"}, nil)
	// sob2cs is the latency from module beginning to current module ending
	som2cs = mon.GetInstance().NewLabeledHistogram("som_cs_latency_ms",
		[]string{"protocol", "topic", "annotation"}, nil)
	// cs2cs is the latency from last module ending to current module ending,
	// cs2cs can be calculated by (cs2sr + som2cs), so not exported.
)

const DeleteMark = "__D__"

func markBaggageItemDeleted(span opentracing.Span, items []string) {
	for _, item := range items {
		span.SetBaggageItem(item, DeleteMark)
	}
}

func removeBaggageItemDeleted(carrier *opentracing.HTTPHeadersCarrier) {
	for k, v := range *carrier {
		if len(v) == 1 && v[0] == DeleteMark {
			delete(*carrier, k)
		}
	}
}

const SpanKey = "span"

func InjectSpanToHeader(span opentracing.Span) map[string]string {
	carrier := opentracing.HTTPHeadersCarrier(make(map[string][]string))
	tracer := opentracing.GlobalTracer()
	if err := tracer.Inject(span.Context(), opentracing.HTTPHeaders, carrier); err != nil {
		return nil
	}
	carrierStr, err := json.Marshal(carrier)
	if err != nil {
		return nil
	}
	return map[string]string{SpanKey: string(carrierStr)}
}

func InjectSpanToHeaderForSending(span opentracing.Span) map[string]string {
	now := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
	span.SetBaggageItem("cs", now)
	if len(span.BaggageItem("sob")) == 0 {
		span.SetBaggageItem("sob", now)
	}
	markBaggageItemDeleted(span, []string{"cr", "sr", "ss", "som"}) // only "cs" and "sob" reserved
	carrier := opentracing.HTTPHeadersCarrier(make(map[string][]string))
	tracer := opentracing.GlobalTracer()
	if err := tracer.Inject(span.Context(), opentracing.HTTPHeaders, carrier); err != nil {
		return nil
	}
	removeBaggageItemDeleted(&carrier)
	carrierStr, err := json.Marshal(carrier)
	if err != nil {
		return nil
	}
	return map[string]string{SpanKey: string(carrierStr)}
}

// export metric sob, som before sending, argument protocal can choose from: kafka|nats|http
func ExportMetricsBeforeSending(span opentracing.Span, metricName string, protocol string) {
	now := time.Now().UnixNano() / 1e6
	if sobStr := span.BaggageItem("sob"); len(sobStr) != 0 {
		sob, err := strconv.ParseInt(sobStr, 10, 0)
		if err == nil {
			sob2cs.WithLabelValues(protocol, metricName, "sob2cs").Observe(float64(now) - float64(sob))
		}
	}
	if somStr := span.BaggageItem("som"); len(somStr) != 0 {
		som, err := strconv.ParseInt(somStr, 10, 0)
		if err == nil {
			som2cs.WithLabelValues(protocol, metricName, "som2cs").Observe(float64(now) - float64(som))
		}
	}
}

func extractSpanFromMsg(msg interface{}) (opentracing.SpanContext, error) {
	switch e := msg.(type) {
	case *kafka.Message:
		for _, header := range e.Headers {
			if header.Key == SpanKey {
				var carrier opentracing.HTTPHeadersCarrier
				err := json.Unmarshal(header.Value, &carrier)
				if err != nil {
					return nil, err
				}
				tracer := opentracing.GlobalTracer()
				// extract span from header
				clientContext, err := tracer.Extract(opentracing.HTTPHeaders, carrier)
				if err != nil {
					return nil, err
				} else {
					return clientContext, nil
				}
			}
		}
	}
	return nil, errors.New("no span")
}

func StartSpanFromMsgAfterReceived(metricName string, msg interface{}) opentracing.Span {
	now := time.Now().UnixNano() / 1e6
	nowStr := fmt.Sprintf("%d", now)
	clientContext, err := extractSpanFromMsg(msg)
	var span opentracing.Span
	if err != nil {
		span = opentracing.StartSpan(metricName)
	} else {
		span = opentracing.StartSpan(metricName, opentracing.FollowsFrom(clientContext))
	}
	if len(span.BaggageItem("sob")) == 0 {
		span.SetBaggageItem("sob", nowStr)
	}
	span.SetBaggageItem("som", nowStr)

	if csStr := span.BaggageItem("cs"); len(csStr) != 0 {
		cs, err := strconv.Atoi(csStr)
		if err == nil {
			cs2sr.WithLabelValues("kafka", metricName, "cs2sr").Observe(float64(now) - float64(cs))
		}
	}
	return span
}

// store a span into context.Context, so it can be passed to other place
func ContextWithSpan(ctx context.Context, span opentracing.Span) context.Context {
	return opentracing.ContextWithSpan(ctx, span)
}

// restore a span from context.Context, so it can be the parent of another child or follow span
func SpanFromContext(ctx context.Context) opentracing.Span {
	return opentracing.SpanFromContext(ctx)
}

// create a follow span from the span stored in context.Context
func StartSobSomSpan(ctx context.Context, operationName string) (opentracing.Span, context.Context) {
	nowStr := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
	span, ctx := StartSpanFromContext(ctx, operationName)
	if len(span.BaggageItem("sob")) == 0 {
		span.SetBaggageItem("sob", nowStr)
	}
	if len(span.BaggageItem("som")) == 0 {
		span.SetBaggageItem("som", nowStr)
	}
	return span, ContextWithSpan(ctx, span)
}

// create a follow span from the span stored in context.Context
func StartSomSpan(ctx context.Context, operationName string) (opentracing.Span, context.Context) {
	nowStr := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
	span, ctx := StartSpanFromContext(ctx, operationName)
	if len(span.BaggageItem("som")) == 0 {
		span.SetBaggageItem("som", nowStr)
	}
	return span, ContextWithSpan(ctx, span)
}

// create a follow span from the span stored in context.Context
func StartSpanFromContext(ctx context.Context, operationName string,
	opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context) {
	var span opentracing.Span
	if parentSpan := SpanFromContext(ctx); parentSpan != nil {
		opts = append(opts, opentracing.FollowsFrom(parentSpan.Context()))
		span = opentracing.StartSpan(operationName, opts...)
	} else {
		span = opentracing.StartSpan(operationName, opts...)
	}
	return span, ContextWithSpan(ctx, span)
}
