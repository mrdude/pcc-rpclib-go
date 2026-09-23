package zipkin

import "net/http"

const (
	traceIdHeader      = "X-B3-TraceId"
	spanIdHeader       = "X-B3-SpanId"
	parentSpanIdHeader = "X-B3-ParentSpanId"

	sampledHeader = "X-B3-Sampled"
	flagsHeader   = "X-B3-Flags"
)

type SpanHeaders struct {
	TraceId      TraceId `json:"traceId"`
	SpanId       SpanId  `json:"id"`
	ParentSpanId *SpanId `json:"parentId,omitempty"`

	Sampled *bool `json:"-"`
	Debug   *bool `json:"debug,omitempty"`
}

func GetSpanHeaders(h http.Header) SpanHeaders {
	sh := SpanHeaders{}
	sh.TraceId = TraceId(h.Get(traceIdHeader))
	sh.SpanId = SpanId(h.Get(spanIdHeader))

	parentSpanIds := h.Values(parentSpanIdHeader)
	if len(parentSpanIds) == 0 {
		sh.ParentSpanId = nil
	} else {
		parentSpanId := SpanId(h.Get(parentSpanIdHeader))
		sh.ParentSpanId = &parentSpanId
	}

	booleanFromHeader := func(headerName string) *bool {
		v := h.Get(headerName)
		if v == "1" {
			sampleValue := true
			return &sampleValue
		} else if v == "0" {
			sampleValue := false
			return &sampleValue
		} else {
			return nil
		}
	}

	sh.Sampled = booleanFromHeader(sampledHeader)
	sh.Debug = booleanFromHeader(flagsHeader)

	return sh
}

func (sh *SpanHeaders) SetHeaders(h http.Header) {
	h.Set(traceIdHeader, string(sh.TraceId))
	h.Set(spanIdHeader, string(sh.SpanId))
	if sh.ParentSpanId == nil {
		h.Del(parentSpanIdHeader)
	} else {
		h.Set(parentSpanIdHeader, string(*sh.ParentSpanId))
	}

	setBoolean := func(headerName string, value *bool) {
		if value == nil {
			return
		}

		if *value == true {
			h.Set(headerName, "1")
		} else if *value == false {
			h.Set(headerName, "0")
		}
	}

	setBoolean(sampledHeader, sh.Sampled)
	setBoolean(flagsHeader, sh.Debug)
}

func (sh *SpanHeaders) IsValid() bool {
	return sh.TraceId != "" && sh.SpanId != "" && (sh.ParentSpanId == nil || *sh.ParentSpanId != "")
}

func (sh *SpanHeaders) IsSampled() bool {
	return (sh.Sampled != nil && *sh.Sampled == true) || (sh.Debug != nil && *sh.Debug == true)
}
