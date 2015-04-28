package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

func Read(r io.Reader, dsts ...interface{}) error {
	var err error
	for _, dst := range dsts {
		switch t := dst.(type) {
		case *int:
			data := make([]byte, 8)
			_, err = r.Read(data)
			if err == nil {
				*t = int(binary.BigEndian.Uint64(data))
			}
		case *byte:
			data := make([]byte, 1)
			_, err = r.Read(data)
			if err == nil {
				*t = data[0]
			}
		case *[]byte:
			var sz int
			err = Read(r, &sz)
			if err == nil {
				buf := bytes.NewBuffer(make([]byte, 0, sz))
				n, err := io.CopyN(buf, r, int64(sz))
				if err != nil {
					return err
				}
				if n != int64(sz) {
					return fmt.Errorf("invalid size")
				}
				*t = buf.Bytes()
			}
		case *string:
			var sz int
			err = Read(r, &sz)
			if err == nil {
				buf := bytes.NewBuffer(make([]byte, 0, sz))
				n, err := io.CopyN(buf, r, int64(sz))
				if err != nil {
					return err
				}
				if n != int64(sz) {
					return fmt.Errorf("invalid size")
				}
				*t = buf.String()
			}
		case *map[string]string:
			var sz int
			err = Read(r, &sz)
			if err == nil {
				m := make(map[string]string, sz)
				for i := 0; i < sz; i++ {
					var k, v string
					err = Read(r, &k, &v)
					if err != nil {
						return err
					}
					m[k] = v
				}
				*t = m
			}
		case *SocketDefinition:
			var flags byte
			err = Read(r, &t.Address, &t.Port, &flags)
			// decode HTTP
			if err == nil {
				if flags&(1<<0) != 0 {
					t.HTTP = new(SocketHTTPDefinition)
					err = Read(r, &t.HTTP.DomainSuffix, &t.HTTP.PathPrefix)
				}
			}
			// decode TLS
			if err == nil {
				if flags&(1<<1) != 0 {
					t.TLS = new(SocketTLSDefinition)
					err = Read(r, &t.TLS.Cert, &t.TLS.Key)
				}
			}
		default:
			err = fmt.Errorf("don't know how to read %T", dst)
		}
		if err != nil {
			break
		}
	}
	return err
}

func Write(w io.Writer, args ...interface{}) error {
	var err error
	for _, arg := range args {
		switch t := arg.(type) {
		case byte:
			data := []byte{t}
			_, err = w.Write(data)
		case int:
			data := make([]byte, 8)
			binary.BigEndian.PutUint64(data, uint64(t))
			_, err = w.Write(data)
		case string:
			err = Write(w, len(t))
			if err == nil {
				_, err = io.CopyN(w, strings.NewReader(t), int64(len(t)))
			}
		case []byte:
			err = Write(w, len(t))
			if err == nil {
				_, err = io.CopyN(w, bytes.NewReader(t), int64(len(t)))
			}
		case map[string]string:
			err = Write(w, len(t))
			if err == nil {
				for k, v := range t {
					err = Write(w, k, v)
					if err != nil {
						return err
					}
				}
			}
		case SocketDefinition:
			var flags byte = 0
			if t.HTTP != nil {
				flags |= 1 << 0
			}
			if t.TLS != nil {
				flags |= 1 << 1
			}
			err = Write(w, t.Address, t.Port, flags)
			if err == nil {
				if t.HTTP != nil {
					err = Write(w, t.HTTP.DomainSuffix, t.HTTP.PathPrefix)
				}
			}
			if err == nil {
				if t.TLS != nil {
					err = Write(w, t.TLS.Cert, t.TLS.Key)
				}
			}
		default:
			err = fmt.Errorf("don't know how to write %T", arg)
		}
	}
	return err
}

func ReadHandshakeRequest(r io.Reader) (HandshakeRequest, error) {
	var req HandshakeRequest
	err := Read(r, &req.SocketDefinition)
	return req, err
}

func WriteHandshakeRequest(w io.Writer, req HandshakeRequest) error {
	return Write(w, req.SocketDefinition)
}

func ReadHandshakeResponse(r io.Reader) (HandshakeResponse, error) {
	var res HandshakeResponse
	err := Read(r, &res.Status)
	return res, err
}

func WriteHandshakeResponse(w io.Writer, res HandshakeResponse) error {
	return Write(w, res.Status)
}
