package apitrace

import "github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"

var nilForwarderInstance = &nilForwarder{}

type nilForwarder struct {
}

func NewNilForwarder() Forwarder {
	return nilForwarderInstance
}

func (f *nilForwarder) SubmitSpan(s zipkin.Span) {
}
