package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/badgerodon/net/socketmaster/protocol"
	"github.com/hashicorp/yamux"
)

type (
	Server struct {
		li       net.Listener
		upstream map[int64]*upstreamListener
		nextID   int64
		config   *Config
		mu       sync.Mutex
	}
)

func New(li net.Listener, cfg *Config) *Server {
	s := &Server{
		li:       li,
		upstream: make(map[int64]*upstreamListener),
		nextID:   1,
		config:   cfg,
	}
	return s
}

func (s *Server) handleDownstreamConnection(conn net.Conn) {
	// a downstream connection starts with a handshake specifying what socket to
	// listen on
	req, err := protocol.ReadHandshakeRequest(conn)
	if err != nil {
		s.config.Logger.Printf("error reading request: %v", err)
		conn.Close()
		return
	}
	protocol.WriteHandshakeResponse(conn, protocol.HandshakeResponse{
		Status: "OK",
	})

	// establish a multiplexed session over the connection
	session, err := yamux.Client(conn, yamux.DefaultConfig())
	if err != nil {
		s.config.Logger.Printf("error reading request: %v", err)
		conn.Close()
		return
	}

	downstream := &downstreamConnection{
		id:               s.nextID,
		session:          session,
		socketDefinition: req.SocketDefinition,
	}
	s.nextID++

	var upstream *upstreamListener
	for _, u := range s.upstream {
		if req.SocketDefinition.Address == u.address &&
			req.SocketDefinition.Port == u.port {
			upstream = u
		}
	}

	if upstream == nil {
		s.config.Logger.Printf("opening new upstream listener: %v:%v", req.SocketDefinition.Address, req.SocketDefinition.Port)
		li, err := net.Listen("tcp", fmt.Sprint(req.SocketDefinition.Address, ":", req.SocketDefinition.Port))
		if err != nil {
			s.config.Logger.Printf("failed to create upstream connection: %v\n", err)
			session.Close()
			return
		}

		upstream = &upstreamListener{
			server:     s,
			id:         s.nextID,
			listener:   li,
			downstream: map[int64]*downstreamConnection{},
			address:    req.SocketDefinition.Address,
			port:       req.SocketDefinition.Port,
		}
		s.nextID++
		s.upstream[upstream.id] = upstream

		go func() {
			for {
				conn, err := li.Accept()
				if err != nil {
					// if this is a temporary error we will try again
					if ne, ok := err.(net.Error); ok && ne.Temporary() {
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}

				go upstream.route(conn)
			}
			upstream.close()
			s.mu.Lock()
			delete(s.upstream, upstream.id)
			s.mu.Unlock()
		}()
	}

	upstream.downstream[downstream.id] = downstream
	upstream.update()
}

func (s *Server) Serve() error {
	upstreamKiller := time.NewTicker(time.Second)
	go func() {
		for range upstreamKiller.C {
			s.mu.Lock()
			for _, u := range s.upstream {
				u.mu.Lock()
				changed := false
				for _, d := range u.downstream {
					if d.session.IsClosed() {
						s.config.Logger.Printf("downstream closed: %v\n", d.session.RemoteAddr())
						delete(u.downstream, d.id)
						changed = true
					}
				}
				u.mu.Unlock()
				if changed {
					u.update()
				}
				u.mu.Lock()
				if len(u.downstream) == 0 &&
					u.lastUpdateTime.After(zeroTime) &&
					u.lastUpdateTime.Add(time.Second*30).Before(time.Now()) {
					go u.close()
					delete(s.upstream, u.id)
				}
				u.mu.Unlock()
			}
			s.mu.Unlock()
		}
	}()
	defer upstreamKiller.Stop()

	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := s.li.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return err
		}

		s.mu.Lock()
		s.handleDownstreamConnection(conn)
		s.mu.Unlock()
	}
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.upstream {
		u.close()
	}
	s.upstream = make(map[int64]*upstreamListener)
	return nil
}
