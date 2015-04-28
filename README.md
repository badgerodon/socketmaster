# socketmaster
A zero-config reverse-proxy written in Go. See https://www.badgerodon.com/socketmaster.

## Installation
Just use standard `go get`:

    go get github.com/badgerodon/socketmaster

## Usage
Run the socketmaster server:

    socketmaster

And write a program that can connect to it:

    package main

    import (
    	"fmt"
    	"io"
    	"net/http"
    	"os"

    	"github.com/badgerodon/socketmaster/client"
    	"github.com/badgerodon/socketmaster/protocol"
    )

    func main() {
    	li, err := client.Listen(protocol.SocketDefinition{
    		Port: 8000,
    	})
    	if err != nil {
    		fmt.Println(err)
    		os.Exit(1)
    	}
    	defer li.Close()

    	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
    		io.WriteString(res, "Hello World")
    	})
    	err = http.Serve(li, nil)
    	if err != nil {
    		fmt.Println(err)
    		os.Exit(1)
    	}
    }

Run that program and you should now be able to reach it:

    curl localhost:8000/test

## Documentation

https://godoc.org/github.com/badgerodon/socketmaster
