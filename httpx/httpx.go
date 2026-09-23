// Package httpx includes a http.Client wrapper that can send requests to UNIX sockets.
package httpx

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	endpoint  ClientEndpoint
	parsedUrl *url.URL

	httpCl *http.Client
}

type OptionsFn func(options *clientOptions)

type clientOptions struct {
	CustomDialer func(ctx context.Context, network, addr string) (net.Conn, error)
	TLSConfig    *tls.Config
}

func WithCustomDialer(dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) OptionsFn {
	return func(opt *clientOptions) {
		opt.CustomDialer = dialContext
	}
}

func WithTLSConfig(tlsCfg *tls.Config) OptionsFn {
	return func(opt *clientOptions) {
		opt.TLSConfig = tlsCfg
	}
}

func NewClient(ep ClientEndpoint, options ...OptionsFn) *Client {
	opts := &clientOptions{}
	for _, fn := range options {
		fn(opts)
	}

	// copy http.DefaultTransport
	tr := http.DefaultTransport.(*http.Transport).Clone()

	tr.TLSClientConfig = opts.TLSConfig
	tr.ForceAttemptHTTP2 = true

	cl := &Client{
		endpoint:  ep,
		parsedUrl: nil,

		httpCl: &http.Client{
			Transport: tr,
		},
	}

	// parse url
	if cl.endpoint.isUnix {
		cl.parsedUrl = &url.URL{
			Scheme: "http",
			Opaque: "",
			User:   nil,
			Host:   "unix-localhost",
			Path:   "",
		}
	} else {
		var err error
		cl.parsedUrl, err = url.Parse(cl.endpoint.ep)
		if err != nil {
			panic(fmt.Errorf("invalid ClientEndpoint URL '%s': %w", cl.endpoint.ep, err))
		}
	}

	// customize http client
	if opts.CustomDialer != nil {
		cl.httpCl.Transport = &http.Transport{
			DialContext:       opts.CustomDialer,
			TLSClientConfig:   opts.TLSConfig,
			ForceAttemptHTTP2: true,
		}
	} else if cl.endpoint.isUnix {
		cl.httpCl.Transport = &http.Transport{
			DialContext:       createUnixDialer(cl.endpoint.socketPath),
			ForceAttemptHTTP2: true,
		}
	}

	return cl
}

func createUnixDialer(socketPath string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return d.DialContext(ctx, "unix", socketPath)
	}
}

func (cl *Client) Endpoint() ClientEndpoint {
	return cl.endpoint
}

// prepare a request URL
// returns the desired Host: header value
func (cl *Client) cleanUrl(u *url.URL) (host string) {
	u.Scheme = cl.parsedUrl.Scheme
	u.Opaque = cl.parsedUrl.Opaque
	u.User = cl.parsedUrl.User
	u.Host = cl.parsedUrl.Host
	host = cl.parsedUrl.Host

	// combine cl.parsedUrl.Path with u.Path
	if cl.parsedUrl.Path == "" || cl.parsedUrl.Path == "/" {
		// newPath = u.Path
	} else {
		// newPath = cl.parsedUrl.Path + u.Path
		p := u.Path
		p = "/" + strings.TrimPrefix(p, "/") // ensure leading /
		u.Path = strings.TrimSuffix(cl.parsedUrl.Path, "/") + p
	}

	return
}

func (cl *Client) Do(req *http.Request) (*http.Response, error) {
	req.Host = cl.cleanUrl(req.URL)
	return cl.httpCl.Do(req)
}
