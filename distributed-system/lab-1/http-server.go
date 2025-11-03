package main

import (
	"fmt"
	"net"
	"os"
	"sync"
)

type server struct {
	wg         sync.WaitGroup
	listener   net.Listener
	shutdonw   chan struct{}
	connection chan net.Conn
}

func newServer(address string) (*server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Failed to listen on address %s : %w", address, err)
	}

	return &server{
		listener:   listener,
		shutdonw:   make(chan struct{}),
		connection: make(chan net.Conn),
	}, nil
}

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Input a valid port number!")
		return
	}

	PORT := ":" + arguments[1]
	s, error := newServer(PORT)
	if error != nil {
		fmt.Println(error)
		os.Exit(1)
	}
}
