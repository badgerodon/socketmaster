package server

import (
	"bytes"
	"io"
	"net"
	"sync"
)

type ReplayConn struct {
	net.Conn
	recording bool
	tmp       bytes.Buffer
	mu        sync.Mutex
}

func NewReplayConn(conn net.Conn) *ReplayConn {
	return &ReplayConn{
		Conn: conn,
	}
}

func (rc *ReplayConn) Record() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.recording = true
	rc.tmp.Reset()
}

func (rc *ReplayConn) Replay() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.recording = false
}

func (rc *ReplayConn) Read(p []byte) (int, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.recording {
		return io.TeeReader(rc.Conn, &rc.tmp).Read(p)
	} else if rc.tmp.Len() > 0 {
		n, err := io.MultiReader(&rc.tmp, rc.Conn).Read(p)
		if rc.tmp.Len() == 0 {
			rc.tmp.Reset()
		}
		return n, err
	} else {
		return rc.Conn.Read(p)
	}
}
