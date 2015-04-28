package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/badgerodon/net/socketmaster/protocol"
	"github.com/hashicorp/yamux"
)

type (
	Router struct {
		listeners  map[int64]*RouterListener
		downstream map[int64]*yamux.Session
		nextID     int64
	}
	RouterListener struct {
		Listener  net.Listener
		Routes    map[int64]Route
		TLSConfig *tls.Config
		IsHTTP    bool
	}
	Route struct {
		SocketDefinition protocol.SocketDefinition
		DownstreamID     int64
	}

	RouterCloseEvent     struct{}
	UpstreamConnectEvent struct {
		ListenerID int64
		Connection net.Conn
		Deadline   time.Time
	}
	UpstreamCloseEvent struct {
		ListenerID int64
		Connection net.Conn
	}
	DownstreamConnectEvent struct {
		SocketDefinition protocol.SocketDefinition
		Connection       *yamux.Session
	}
	DownstreamCloseEvent struct {
		DownstreamID int64
	}
	ListenerCloseEvent struct {
		ListenerID int64
	}
)

func NewRouter() *Router {
	return &Router{
		listeners:  make(map[int64]*RouterListener),
		downstream: make(map[int64]*yamux.Session),
		nextID:     1,
	}
}

func (r *Router) match(rli *RouterListener, conn net.Conn) (Route, bool) {
	for _, route := range rli.Routes {
		return route, true
	}
	return Route{}, false
}

func (r *Router) handleDownstreamClose(events chan interface{}, evt DownstreamCloseEvent) {
	for id, rli := range r.listeners {
		for routeID, route := range rli.Routes {
			if route.DownstreamID == evt.DownstreamID {
				delete(rli.Routes, routeID)
			}
		}
		log.Println("ROUTES:", rli.Routes)
		if len(rli.Routes) == 0 {
			rli.Listener.Close()
			events <- ListenerCloseEvent{ListenerID: id}
		}
	}
}

func (r *Router) handleDownstreamConnect(events chan interface{}, evt DownstreamConnectEvent) {
	downstreamID := r.nextID
	r.nextID++
	r.downstream[downstreamID] = evt.Connection

	var listenerID int64

	var rli *RouterListener
	for id, trli := range r.listeners {
		for _, route := range trli.Routes {
			if route.SocketDefinition.Address == evt.SocketDefinition.Address &&
				route.SocketDefinition.Port == evt.SocketDefinition.Port {
				rli = trli
				listenerID = id
			}
		}
	}

	if listenerID == 0 {
		listenerID := r.nextID
		r.nextID++

		rli = &RouterListener{
			Routes: make(map[int64]Route),
		}
		r.listeners[listenerID] = rli

		li, err := net.Listen("tcp", fmt.Sprint(evt.SocketDefinition.Address, ":", evt.SocketDefinition.Port))
		if err != nil {
			log.Printf("[socketmaster] error listening on %v\n", err)
			events <- ListenerCloseEvent{
				ListenerID: listenerID,
			}
			return
		}
		go func() {
			defer func() {
				li.Close()
				events <- ListenerCloseEvent{
					ListenerID: listenerID,
				}
			}()
			for {
				conn, err := li.Accept()
				if err != nil {
					if ne, ok := err.(net.Error); ok && ne.Temporary() {
						time.Sleep(time.Second * 1)
						continue
					}
					break
				}
				events <- UpstreamConnectEvent{
					ListenerID: listenerID,
					Connection: conn,
					Deadline:   time.Now().Add(time.Second * 30),
				}
			}
		}()
		rli.Listener = li
	}

	routeID := r.nextID
	r.nextID++

	rli.Routes[routeID] = Route{
		SocketDefinition: evt.SocketDefinition,
		DownstreamID:     downstreamID,
	}
	// determine if we should use HTTP
	rli.IsHTTP = false
	for _, route := range rli.Routes {
		if route.SocketDefinition.HTTP != nil {
			rli.IsHTTP = true
		}
	}
	// setup the TLS config (if necessary)
	rli.TLSConfig = nil
	certs := make([]tls.Certificate, 0)
	for _, route := range rli.Routes {
		if route.SocketDefinition.TLS != nil {
			cert, err := tls.X509KeyPair([]byte(route.SocketDefinition.TLS.Cert), []byte(route.SocketDefinition.TLS.Key))
			if err != nil {
				log.Printf("[socketmaster] error loading certificate: %v\n", err)
				continue
			}
			certs = append(certs, cert)
		}
	}
	if len(certs) > 0 {
		rli.TLSConfig = &tls.Config{
			Certificates: certs,
		}
	}
}

func (r *Router) handleUpstreamConnect(events chan interface{}, evt UpstreamConnectEvent) {
	conn := evt.Connection
	if time.Now().After(evt.Deadline) {
		go conn.Close()
		return
	}

	rli, ok := r.listeners[evt.ListenerID]
	if !ok {
		log.Printf("[socketmaster] failed to find listener: %v\n", evt.ListenerID)
		go conn.Close()
		return
	}

	if rli.TLSConfig != nil {
		conn = tls.Server(conn, rli.TLSConfig)
	}

	route, ok := r.match(rli, conn)
	if !ok {
		log.Printf("[socketmaster] no matching route for: %v\n", conn)
		time.AfterFunc(time.Second, func() {
			events <- evt
		})
		return
	}

	d, ok := r.downstream[route.DownstreamID]
	if !ok {
		log.Printf("[socketmaster] failed to find downstream connection: %v\n", route.DownstreamID)
		go conn.Close()
		return
	}

	stream, err := d.OpenStream()
	if err != nil {
		log.Printf("[socketmaster] failed to open stream: %v\n", err)
		evt.Connection.Close()
		d.Close()
		events <- DownstreamCloseEvent{
			DownstreamID: route.DownstreamID,
		}
		return
	}

	go func() {
		signal := make(chan struct{}, 2)
		go func() {
			io.Copy(stream, evt.Connection)
			signal <- struct{}{}
		}()
		go func() {
			io.Copy(evt.Connection, stream)
			signal <- struct{}{}
		}()
		<-signal
		evt.Connection.Close()
		stream.Close()
		events <- UpstreamCloseEvent{
			ListenerID: evt.ListenerID,
			Connection: evt.Connection,
		}
	}()
}

func (r *Router) handleListenerClose(events chan interface{}, evt ListenerCloseEvent) {
	rli, ok := r.listeners[evt.ListenerID]
	if ok {
		delete(r.listeners, evt.ListenerID)
		for _, route := range rli.Routes {
			did := route.DownstreamID
			if d, ok := r.downstream[did]; ok {
				delete(r.downstream, did)
				go func() {
					d.Close()
					events <- DownstreamCloseEvent{
						DownstreamID: did,
					}
				}()
			}
		}
	}
}

func (r *Router) handleClose(events chan interface{}, evt RouterCloseEvent) {
	for id, _ := range r.listeners {
		listenerID := id
		rli := r.listeners[id]
		go func() {
			rli.Listener.Close()
			events <- ListenerCloseEvent{
				ListenerID: listenerID,
			}
		}()
	}
}

func (r *Router) Route(events chan interface{}) {
	for evt := range events {
		log.Printf("[router] %T: %v\n", evt, evt)
		switch t := evt.(type) {
		case DownstreamCloseEvent:
			r.handleDownstreamClose(events, t)
		case DownstreamConnectEvent:
			r.handleDownstreamConnect(events, t)
		case ListenerCloseEvent:
			r.handleListenerClose(events, t)
		case UpstreamConnectEvent:
			r.handleUpstreamConnect(events, t)
		case UpstreamCloseEvent:
		case RouterCloseEvent:
			r.handleClose(events, t)
		}
	}
}
