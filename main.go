package main

import (
	"flag"
	"log"
	"net"

	"github.com/badgerodon/net/socketmaster/server"
)

var (
	bind = flag.String("bind", "127.0.0.1:9999", "address to accept downstream connections")
)

func main() {
	log.SetFlags(0)
	flag.Parse()
	log.Println("[socketmaster] starting server on", *bind)
	li, err := net.Listen("tcp", *bind)
	if err != nil {
		log.Fatalln(err)
	}
	defer li.Close()

	s := server.New(li, server.DefaultConfig())
	defer s.Close()
	s.Serve()
}
