package protocol

type (
	SocketHTTPDefinition struct {
		DomainSuffix, PathPrefix string
	}
	SocketTLSDefinition struct {
		Cert, Key string
	}
	SocketDefinition struct {
		Address string
		Port    int
		TLS     *SocketTLSDefinition
		HTTP    *SocketHTTPDefinition
	}
	HandshakeRequest struct {
		SocketDefinition SocketDefinition
	}
	HandshakeResponse struct {
		Status string
	}
)
