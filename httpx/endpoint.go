package httpx

type ClientEndpoint struct {
	isUnix bool // is this a unix socket endpoint?

	ep         string // for Inet endpoints: the endpoint
	socketPath string // for UNIX endpoints: the unix socket path
}

func (ep ClientEndpoint) String() string {
	if ep.isUnix {
		return "unix://" + ep.socketPath
	}

	return ep.ep
}

func UnixEndpoint(socketPath string) ClientEndpoint {
	return ClientEndpoint{
		isUnix:     true,
		socketPath: socketPath,
	}
}

func InetEndpoint(hostport string) ClientEndpoint {
	return ClientEndpoint{
		isUnix: false,
		ep:     hostport,
	}
}
