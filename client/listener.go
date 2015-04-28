package client

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/badgerodon/socketmaster/protocol"
	"github.com/hashicorp/yamux"
)

type Listener struct {
	socketMasterAddress string
	socketDefinition    protocol.SocketDefinition
	session             *yamux.Session
	mu                  sync.Mutex
}

func (li *Listener) getSession() (*yamux.Session, error) {
	li.mu.Lock()
	defer li.mu.Unlock()

	if li.session != nil {
		return li.session, nil
	}

	// connect to the socket master
	conn, err := net.Dial("tcp", li.socketMasterAddress)
	if err != nil {
		return nil, err
	}

	// bind to a port
	err = protocol.WriteHandshakeRequest(conn, protocol.HandshakeRequest{
		SocketDefinition: li.socketDefinition,
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

func (li *Listener) Accept() (net.Conn, error) {
	deadline := time.Now().Add(time.Second * 30)
	for time.Now().Before(deadline) {
		session, err := li.getSession()
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}

		conn, err := session.Accept()
		if err != nil {
			li.mu.Lock()
			li.session = nil
			li.mu.Unlock()
			session.Close()
			time.Sleep(time.Second * 1)
			continue
		}
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("failed to get connection")
}

func (li *Listener) Close() error {
	panic("not implemented")
}

func (li *Listener) Addr() net.Addr {
	panic("not implemented")
}
