package zipkin

import (
	"net/netip"
	"time"
)

type Kind string

const (
	ClientKind   Kind = "CLIENT"
	ServerKind   Kind = "SERVER"
	ProducerKind Kind = "PRODUCER"
	ConsumerKind Kind = "CONSUMER"
)

const (
	ServerSendAnnotation = "ss"
	ServerRecvAnnotation = "sr"
	ClientSendAnnotation = "cs"
	ClientRecvAnnotation = "cr"
)

type Span struct {
	SpanHeaders

	Name string `json:"name"`

	Ts       Time     `json:"timestamp"` //in epoch microseconds
	Duration Duration `json:"duration"`  //in microseconds

	Kind Kind `json:"kind"`

	Shared bool `json:"shared"`

	LocalEndpoint  *Endpoint `json:"localEndpoint,omitempty"`
	RemoteEndpoint *Endpoint `json:"remoteEndpoint,omitempty"`

	Annotations []Annotation      `json:"annotations,omitempty"`
	Tags        map[string]string `json:"tags"`
}

type Endpoint struct {
	ServiceName *string `json:"serviceName,omitempty"`

	IPv4 *string `json:"ipv4,omitempty"`
	IPv6 *string `json:"ipv6,omitempty"`

	Port *int32 `json:"port,omitempty"`
}

func (ep *Endpoint) SetService(service string) {
	ep.ServiceName = &service
}

func (ep *Endpoint) SetAddrPort(ip netip.AddrPort) {
	if ip.Addr().Is4() {
		ipStr := ip.Addr().String()
		ep.IPv4 = &ipStr
	} else if ip.Addr().Is6() {
		ipStr := ip.Addr().String()
		ep.IPv6 = &ipStr
	}

	ep.Port = new(int32(ip.Port()))
}

type Annotation struct {
	Value string `json:"value"`
	Ts    Time   `json:"timestamp"` //in epoch microseconds
}

func (s *Span) AddAnnotation(value string) {
	s.Annotations = append(s.Annotations, Annotation{
		Value: value,
		Ts:    Time{Time: time.Now()},
	})
}

func (s *Span) Clone() Span {
	spanCopy := *s

	// deep copy of slices and pointers
	if spanCopy.LocalEndpoint != nil {
		ep := *spanCopy.LocalEndpoint
		spanCopy.LocalEndpoint = &ep
	}

	if spanCopy.RemoteEndpoint != nil {
		ep := *spanCopy.RemoteEndpoint
		spanCopy.RemoteEndpoint = &ep
	}

	if len(spanCopy.Annotations) > 0 {
		spanCopy.Annotations = nil

		for _, a := range s.Annotations {
			spanCopy.Annotations = append(spanCopy.Annotations, a)
		}
	}

	if len(spanCopy.Tags) > 0 {
		spanCopy.Tags = make(map[string]string)
		for k, v := range s.Tags {
			spanCopy.Tags[k] = v
		}
	}

	return spanCopy
}
