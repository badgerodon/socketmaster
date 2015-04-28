package client

import (
	"fmt"
	"net"

	"github.com/badgerodon/socketmaster/protocol"
	"github.com/hashicorp/yamux"
)

var DefaultSocketMasterAddress = "127.0.0.1:9999"

type Client struct {
	socketMasterAddress string
}

func New(socketMasterAddress string) *Client {
	return &Client{socketMasterAddress}
}

// Listen connects to the socket master, binds a port, and accepts
// multiplexed traffic as new connections
func (client *Client) Listen(socketDefinition protocol.SocketDefinition) (net.Listener, error) {
	// connect to the socket master
	conn, err := net.Dial("tcp", client.socketMasterAddress)
	if err != nil {
		return nil, err
	}

	// bind to a port
	err = protocol.WriteHandshakeRequest(conn, protocol.HandshakeRequest{
		SocketDefinition: socketDefinition,
	})
	if err != nil {
		conn.Close()
		return nil, err
	}

	// see if that worked
	res, err := protocol.ReadHandshakeResponse(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if res.Status != "OK" {
		conn.Close()
		return nil, fmt.Errorf("%s", res.Status)
	}

	// start a new session
	session, err := yamux.Server(conn, yamux.DefaultConfig())
	if err != nil {
		conn.Close()
		return nil, err
	}

	return session, nil
}

func Listen(socketDefinition protocol.SocketDefinition) (net.Listener, error) {
	return New(DefaultSocketMasterAddress).Listen(socketDefinition)
}
