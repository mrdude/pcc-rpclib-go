package apitrace

import "github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"

// Forwarder submits zipkin.Span's to a remote site
type Forwarder interface {
	SubmitSpan(s zipkin.Span)
}
