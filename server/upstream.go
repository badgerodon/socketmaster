package server

import (
	"bufio"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/badgerodon/net/socketmaster/protocol"
	"github.com/hashicorp/yamux"
)

var zeroTime time.Time

type (
	upstreamListener struct {
		server         *Server
		id             int64
		listener       net.Listener
		downstream     map[int64]*downstreamConnection
		address        string
		port           int
		tlsConfig      *tls.Config
		lastUpdateTime time.Time
		mu             sync.RWMutex
	}
	downstreamConnection struct {
		id               int64
		session          *yamux.Session
		socketDefinition protocol.SocketDefinition
	}
)

func (u *upstreamListener) closeDownstream(id int64) {
	u.mu.Lock()
	d, ok := u.downstream[id]
	if ok {
		d.session.Close()
	}
	u.mu.Unlock()

	u.update()
}

func (u *upstreamListener) findDownstream(req *http.Request) *downstreamConnection {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, d := range u.downstream {
		if d.socketDefinition.HTTP != nil {
			if strings.HasSuffix(req.Host, d.socketDefinition.HTTP.DomainSuffix) &&
				strings.HasPrefix(req.URL.Path, d.socketDefinition.HTTP.PathPrefix) {
				return d
			}
		}
	}

	return nil
}

func (u *upstreamListener) routeHTTP(conn net.Conn) {
	defer conn.Close()

	var lastStream *yamux.Stream
	var lastSession *yamux.Session
	defer func() {
		if lastStream != nil {
			lastStream.Close()
		}
	}()

	for {
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}

		deadline := time.Now().Add(time.Second * 30)
		for {
			d := u.findDownstream(req)
			if d == nil {
				if time.Now().After(deadline) {
					msg := "Not Found"
					err = (&http.Response{
						Status:        "404 Not Found",
						StatusCode:    404,
						Proto:         "HTTP/1.1",
						ProtoMajor:    1,
						ProtoMinor:    1,
						Body:          ioutil.NopCloser(strings.NewReader(msg)),
						ContentLength: int64(len(msg)),
						Request:       req,
					}).Write(conn)
					return
				} else {
					time.Sleep(time.Millisecond * 100)
					continue
				}
			}

			if d.session != lastSession && lastStream != nil {
				lastStream.Close()
			}
			lastSession = d.session

			lastStream, err = lastSession.OpenStream()
			if err != nil {
				lastSession = nil
				lastStream = nil

				d.session.Close()
				u.mu.Lock()
				delete(u.downstream, d.id)
				u.mu.Unlock()
				u.update()
				continue
			}
			break
		}

		err = req.Write(lastStream)
		if err != nil {
			return
		}
		res, err := http.ReadResponse(bufio.NewReader(lastStream), req)
		if err != nil {
			return
		}
		err = res.Write(conn)
		if err != nil {
			return
		}
	}
}

func (u *upstreamListener) route(conn net.Conn) {
	u.mu.RLock()
	if u.tlsConfig != nil {
		conn = tls.Server(conn, u.tlsConfig)
	}
	useHTTP := false
	candidates := make([]*downstreamConnection, 0, len(u.downstream))
	for _, d := range u.downstream {
		candidates = append(candidates, d)
		if d.socketDefinition.HTTP != nil {
			useHTTP = true
		}
	}
	u.mu.RUnlock()

	if useHTTP {
		go u.routeHTTP(conn)
	} else {
		var stream *yamux.Stream
		var err error

		for {
			if len(candidates) == 0 {
				conn.Close()
				return
			}
			d := candidates[0]
			stream, err = d.session.OpenStream()
			if err != nil {
				u.server.config.Logger.Printf("failed to open stream: %v\n", err)
				d.session.Close()
				u.mu.Lock()
				delete(u.downstream, d.id)
				u.mu.Unlock()
				u.update()
				continue
			}
			break
		}

		go func() {
			signal := make(chan struct{}, 2)
			go func() {
				io.Copy(stream, conn)
				signal <- struct{}{}
			}()
			go func() {
				io.Copy(conn, stream)
				signal <- struct{}{}
			}()
			<-signal
			conn.Close()
			stream.Close()
		}()
	}
}

func (u *upstreamListener) update() {
	u.mu.Lock()
	defer u.mu.Unlock()

	// rebuild the TLS config
	certs := make([]tls.Certificate, 0)
	for _, d := range u.downstream {
		if d.socketDefinition.TLS != nil {
			cert, err := tls.X509KeyPair([]byte(d.socketDefinition.TLS.Cert), []byte(d.socketDefinition.TLS.Key))
			if err == nil {
				certs = append(certs, cert)
			} else {
				u.server.config.Logger.Printf("failed to load tls cert: %v\n", err)
			}
		}
	}
	if len(certs) > 0 {
		u.tlsConfig = &tls.Config{Certificates: certs}
		u.tlsConfig.BuildNameToCertificate()
	} else {
		u.tlsConfig = nil
	}

	u.server.config.Logger.Printf("updated upstream connection: %v\n", u.listener.Addr())
	for _, d := range u.downstream {
		u.server.config.Logger.Printf("- downstream: %v\n", d.session.RemoteAddr())
	}
	if u.tlsConfig == nil {
		u.server.config.Logger.Printf("- tls disabled\n")
	} else {
		u.server.config.Logger.Printf("- tls enabled\n")
	}

	u.lastUpdateTime = time.Now()
}

func (u *upstreamListener) close() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.listener != nil {
		u.server.config.Logger.Printf("closing upstream connection: %v\n", u.listener.Addr())
		u.listener.Close()
		u.listener = nil
	}
}
