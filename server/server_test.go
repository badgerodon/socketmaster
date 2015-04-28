package server

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/badgerodon/net/socketmaster/client"
	"github.com/badgerodon/net/socketmaster/protocol"
	"github.com/hashicorp/yamux"
)

var (
	tlsCert = `
-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAKaMzubIBJ4yMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTUwNDI0MjE0NDI4WhcNMTYwNDIzMjE0NDI4WjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEAxlmx+DrHORae4MmkphG29+HjqvGOb0Exc5h2EmFgbk4Dera7aKT6OzyZ
yw/2pFdpK82MyomdevI5Me/Bg3z1wWij6NalMaOvMq/SKjf6CkeORoRaoWPkOOQa
XuSAlDsevUDNuF+73JaVkLHZdCUMP4Uz0Jq5U0tRodmxI2ntf5sYXtn1RHbgRT4v
LjHLQwALiv6XbfRg0RFZxVmYa+9powOJvUauddJpMLQi8AgRnv7H6PjTbYafPejy
4EbhoFUFlX87+kjD4GigAqSStEar2S+OzNIQeIL1+B6tilpJ4qRQfVS/GNjvNNtZ
xg35Vcvn/WOXKETii0W0XY6rgv4YSQIDAQABo1AwTjAdBgNVHQ4EFgQUNkpbiKTN
LrXTqMy8lmg51DEVpWAwHwYDVR0jBBgwFoAUNkpbiKTNLrXTqMy8lmg51DEVpWAw
DAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAcxr5+aDAKRPP/hf/hfBS
xuXKRFAPwoAfQ6GqB/g1ZeDfdGITpfhBqkRPZz7Ldq1BTcpkPwP5umKv2HHTYdzI
5wR4NkSCePtLb05KiYkuT90ytqlbT6oaiEOdrlK/0kvRKteYld+O4W31SXNVSxt0
R21b5iPkdQheWsMBNmV7ZS+UzUo9I4IHrWlccKuwWALDcUWIjYpxkTSRKNUj2Ros
Go1mC/AIgdDj2shwIVpIeN+98qef0IqgO+xSdFLZ1k5d2TQhv3ZdF32px3zFuP8O
95ZPpE6Q2ovQbU5N4hj+uLqiyh42V4HUJJNsOFHx0jX+NNAYsRPxpcEBVZbGghlQ
5Q==
-----END CERTIFICATE-----
`
	tlsKey = `
-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQDGWbH4Osc5Fp7g
yaSmEbb34eOq8Y5vQTFzmHYSYWBuTgN6trtopPo7PJnLD/akV2krzYzKiZ168jkx
78GDfPXBaKPo1qUxo68yr9IqN/oKR45GhFqhY+Q45Bpe5ICUOx69QM24X7vclpWQ
sdl0JQw/hTPQmrlTS1Gh2bEjae1/mxhe2fVEduBFPi8uMctDAAuK/pdt9GDREVnF
WZhr72mjA4m9Rq510mkwtCLwCBGe/sfo+NNthp896PLgRuGgVQWVfzv6SMPgaKAC
pJK0RqvZL47M0hB4gvX4Hq2KWknipFB9VL8Y2O8021nGDflVy+f9Y5coROKLRbRd
jquC/hhJAgMBAAECggEADH/miUAbAev9Aylx6M1A/IoNsN4cHcK7/Q7kke/1Bb6A
1aDiWovbARSmlHdjEaQ4inwfnTvi4raVCCKVzVV4n0Ga4rd0HZa1Gbqewe5ZIYC0
5Ji+pWEIJtWpG8XGnJDFNSP6Ut4llpcewcmTbJBRH0ejpke52hfrAwoW8aZhQyNE
hkfdlRQ6f1unk+p1It+nGk65cvZkxkPzgo8k7iCsZL8WBHXMpqiHk3Hnmly4LIM2
K6S996Q3kWp2qGUj0BooeL0Q139Me4g2UloiWx/Jyr0M6tOGEEAua0ncmcpSeopM
GOXTchpWm6uJ5Q0AvIARdby3EMTYqUvzIEmkqlJhAQKBgQDwPXdxFMblj+kTdk4y
aYGiWM13yNpFVqnE/SrlZlkd5lNrH7XJl52KptMOdF5ejC9tHZebTLlvDHbKi9KE
ZLtTe+4tFf4k0XTscMZKsX35s4YOM6KglZ9Enq9Y6Taiv4UCXTwhJ2lkCqlZevpB
mdGgvzEQOixadTQNk5IqjKESGQKBgQDTXL4jradT3ns7urz6FZIfrwqVb83hqrVT
K3FNqKIqLUktJfIEsz1I6EDYN3V5eX76qfmaiYuoGIabsZG7f2oOJ5qhRDx9aSjk
4BDRSu0WKLWMgdn3Rst6F1dJfl7FNWwA8P3xIcw61MUuf3YKNKzsQqZEJEbO3WBY
y+wUnSndsQKBgBKwFVx8i0RMP4s+BrMxNd5VHhaVTzVZmncyYmXZ4lDLG+4XV2LJ
In4on/5d2wFr0jygsqxn+XzD8XGsEsItu8ywtURYk551lKzX0PT6fZww9Nqh9aKc
QPlrhqRZ7+AVGdmnOwgxMqePlMDbKiB0QLRKaxyiCdU3jMcJlbMtoVHxAoGAXrER
GkjlSyTEjwjlOyFI2tr/4d06HpztKXqwAzvGkyDAxPJYEBUBItWyn2uRPL/azJA0
HDD9GW0LeVs/UAIQUJEbrJ42f3UKdieQQUPRHflVBML0FN1psaQdXfa4nJ+HaJCP
JGWg6saCJIfEKWRaCGr/tE2QT4NMc9vAQ6f5prECgYBiXf4czXtMjSxbNxgVtES0
waQ15rLrV0tGS93Aaw4wyC0JCnwYNFp85TJUUQtLGs+K5CFa0qN73hcrm/S17WYn
P3ae2cxUQewbSm2HWY+LX+xdRpqXOdDxlS+AAVeLn9/Fey+puzAseFcaxHRzAUja
nZby8o0vTsy3uUVhYrxJyg==
-----END PRIVATE KEY-----
`
)

func httpGet(url string) string {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Get(url)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	if res.StatusCode != 200 {
		return "ERROR: " + res.Status
	}
	defer res.Body.Close()
	bs, _ := ioutil.ReadAll(res.Body)
	return string(bs)
}

func Test(t *testing.T) {
	li1, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Errorf("error listening: %v", err)
		return
	}
	defer li1.Close()

	s := New(li1, DefaultConfig())
	defer s.Close()
	go s.Serve()

	c1, err := net.Dial("tcp", li1.Addr().String())
	if err != nil {
		t.Errorf("error dialing: %v", err)
		return
	}
	defer c1.Close()

	err = protocol.WriteHandshakeRequest(c1, protocol.HandshakeRequest{
		SocketDefinition: protocol.SocketDefinition{
			Address: "127.0.0.1",
			Port:    8999,
		},
	})
	if err != nil {
		t.Errorf("error writing handshake: %v", err)
		return
	}
	res, err := protocol.ReadHandshakeResponse(c1)
	if err != nil {
		t.Errorf("error reading handshake: %v", err)
		return
	}
	if res.Status != "OK" {
		t.Errorf("expected `OK` got `%v`", res.Status)
		return
	}

	session, err := yamux.Server(c1, yamux.DefaultConfig())
	if err != nil {
		t.Errorf("error creating session: %v", err)
		return
	}
	defer session.GoAway()

	n := 5
	ch := make(chan string, n)
	go func() {
		for i := 0; i < n; i++ {
			stream, err := session.AcceptStream()
			if err != nil {
				t.Errorf("error creating session: %v", err)
				ch <- ""
				return
			}

			var buf bytes.Buffer
			io.Copy(&buf, stream)
			ch <- buf.String()
			stream.Close()
		}
	}()

	time.Sleep(time.Millisecond * 50)

	go func() {
		for i := 0; i < n; i++ {
			c2, err := net.Dial("tcp", "127.0.0.1:8999")
			if err != nil {
				t.Errorf("failed to connect: %v", err)
				return
			}
			io.WriteString(c2, "Hello World")
			c2.Close()
		}
	}()

	e := "Hello World"
	for i := 0; i < n; i++ {
		v := <-ch
		if e != v {
			t.Errorf("expected `%v` got `%v`", e, v)
			return
		}
	}
}

func TestHTTP(t *testing.T) {
	li1, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Errorf("error listening: %v", err)
		return
	}
	s := New(li1, DefaultConfig())
	defer s.Close()
	go s.Serve()

	c1, err := client.Listen(li1.Addr().String(), protocol.SocketDefinition{
		Address: "127.0.0.1",
		Port:    8999,
		HTTP: &protocol.SocketHTTPDefinition{
			DomainSuffix: "",
			PathPrefix:   "/a/",
		},
	})
	if err != nil {
		t.Errorf("error dialing: %v", err)
		return
	}
	defer c1.Close()

	go http.Serve(c1, http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		io.WriteString(res, "a")
	}))

	c2, err := client.Listen(li1.Addr().String(), protocol.SocketDefinition{
		Address: "127.0.0.1",
		Port:    8999,
		HTTP: &protocol.SocketHTTPDefinition{
			DomainSuffix: "",
			PathPrefix:   "/b/",
		},
	})
	if err != nil {
		t.Errorf("error dialing: %v", err)
		return
	}
	defer c2.Close()

	go http.Serve(c2, http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		io.WriteString(res, "b")
	}))

	time.Sleep(50 * time.Millisecond)

	// should go to the first server
	str := httpGet("http://127.0.0.1:8999/a/")
	if str != "a" {
		t.Error("expected `a` got", str)
		return
	}
	// should go to the second server
	str = httpGet("http://127.0.0.1:8999/b/")
	if str != "b" {
		t.Error("expected `b` got", str)
		return
	}
}

func TestTLS(t *testing.T) {
	li1, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Errorf("error listening: %v", err)
		return
	}
	s := New(li1, DefaultConfig())
	defer s.Close()
	go s.Serve()

	c1, err := client.Listen(li1.Addr().String(), protocol.SocketDefinition{
		Address: "127.0.0.1",
		Port:    8999,
		HTTP: &protocol.SocketHTTPDefinition{
			DomainSuffix: "",
			PathPrefix:   "/a/",
		},
		TLS: &protocol.SocketTLSDefinition{
			Cert: tlsCert,
			Key:  tlsKey,
		},
	})
	if err != nil {
		t.Errorf("error dialing: %v", err)
		return
	}
	defer c1.Close()

	go http.Serve(c1, http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		io.WriteString(res, "a")
	}))

	str := httpGet("https://127.0.0.1:8999/a/")
	if str != "a" {
		t.Error("expected `a` got", str)
		return
	}
}
