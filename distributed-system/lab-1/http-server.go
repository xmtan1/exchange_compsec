package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const maxConnection = 10

type server struct {
	connectionCount int
	wg              sync.WaitGroup
	listener        net.Listener
	shutdown        chan struct{}
	connection      chan net.Conn
}

func newServer(address string) (*server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on address %s : %w", address, err)
	}

	return &server{
		connectionCount: 0,
		listener:        listener,
		shutdown:        make(chan struct{}),
		connection:      make(chan net.Conn),
	}, nil
}

// handle the "accept connection phase"
func (s *server) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				continue
			}
			s.connection <- conn
			s.connectionCount += 1
		}
	}
}

// pick a connection in "Connections" to serve
func (s *server) handleConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			return
		case conn := <-s.connection:
			go s.handleConnection(conn)
		}
	}
}

func (s *server) handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Fprintf(conn, "TCP server accepted a connection, total connection: %d", s.connectionCount)
	time.Sleep(5 * time.Second)
	s.connectionCount -= 1
	fmt.Fprintf(conn, "Handling connection completed, remaining %d", s.connectionCount)
}

func (s *server) Start() {
	s.wg.Add(2)
	go s.acceptConnections()
	go s.handleConnections()
}

func (s *server) Stop() {
	close(s.shutdown)
	s.listener.Close()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(time.Second):
		fmt.Println("Time out waiting remaining connections to be finished.")
		return
	}
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

	s.Start()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down the server ...")
	s.Stop()
	fmt.Println("Server stopped.")
}
