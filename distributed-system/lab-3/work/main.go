package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	pb "chord/protocol" // Update path as needed

	"google.golang.org/grpc"
)

var localaddress string

// Find our local IP address
func init() {
	// Configure log package to show short filename, line number and timestamp with only time
	log.SetFlags(log.Lshortfile | log.Ltime)

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	localaddress = localAddr.IP.String()

	if localaddress == "" {
		panic("init: failed to find non-loopback interface with valid address on this node")
	}
	log.Printf("found local address %s\n", localaddress)
}

// resolveAddress handles :port format by adding the local address
func resolveAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return net.JoinHostPort(localaddress, address[1:])
	} else if !strings.Contains(address, ":") {
		return net.JoinHostPort(address, defaultPort)
	}
	return address
}

// StartServer starts the gRPC server for this node
func StartServer(address string, nprime string) (*Node, error) {
	address = resolveAddress(address)

	node := &Node{
		Address:     address,
		FingerTable: make([]string, keySize+1),
		Predecessor: "",
		Successors:  nil,
		Bucket:      make(map[string]string),
	}

	// Are we the first node?
	if nprime == "" {
		log.Print("StartServer: creating new ring")
		node.Successors = []string{node.Address}
	} else {
		log.Print("StartServer: joining existing ring using ", nprime)
		// For now use the given address as our successor
		nprime = resolveAddress(nprime)
		node.Successors = []string{nprime}
		// TODO: use a GetAll request to populate our bucket
	}

	// Start listening for RPC calls
	grpcServer := grpc.NewServer()
	pb.RegisterChordServer(grpcServer, node)

	lis, err := net.Listen("tcp", node.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err)
	}

	// Start server in goroutine
	log.Printf("Starting Chord node server on %s", node.Address)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Start background tasks
	go func() {
		nextFinger := 0
		for {
			time.Sleep(time.Second / 3)
			node.stabilize()

			time.Sleep(time.Second / 3)
			nextFinger = node.fixFingers(nextFinger)

			time.Sleep(time.Second / 3)
			node.checkPredecessor()
		}
	}()

	return node, nil
}

// RunShell provides an interactive command shell
func RunShell(node *Node) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nExiting...")
				return
			}
			fmt.Println("Error reading input:", err)
			continue
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		switch parts[0] {
		case "help":
			fmt.Println("Available commands:")
			fmt.Println("  help              - Show this help message")
			fmt.Println("  ping <address>    - Ping another node")
			fmt.Println("                      (You can use :port for localhost)")
			fmt.Println("  put <key> <value> <address> - Store a key-value pair on a node")
			fmt.Println("  get <key> <address>         - Get a value for a key from a node")
			fmt.Println("  delete <key> <address>      - Delete a key from a node")
			fmt.Println("  getall <address>            - Get all key-value pairs from a node")
			fmt.Println("  dump              - Display info about the current node")
			fmt.Println("  quit              - Exit the program")

		case "ping":
			if len(parts) < 2 {
				fmt.Println("Usage: ping <address>")
				continue
			}

			err := PingNode(ctx, parts[1])
			if err != nil {
				fmt.Printf("Ping failed: %v\n", err)
			} else {
				fmt.Println("Ping successful")
			}

		case "put":
			if len(parts) < 4 {
				fmt.Println("Usage: put <key> <value> <address>")
				continue
			}

			err := PutKeyValue(ctx, parts[1], parts[2], parts[3])
			if err != nil {
				fmt.Printf("Put failed: %v\n", err)
			} else {
				fmt.Printf("Put successful: %s -> %s\n", parts[1], parts[2])
			}

		case "get":
			if len(parts) < 3 {
				fmt.Println("Usage: get <key> <address>")
				continue
			}

			value, err := GetValue(ctx, parts[1], parts[2])
			if err != nil {
				fmt.Printf("Get failed: %v\n", err)
			} else if value == "" {
				fmt.Printf("Key '%s' not found\n", parts[1])
			} else {
				fmt.Printf("%s -> %s\n", parts[1], value)
			}

		case "delete":
			if len(parts) < 3 {
				fmt.Println("Usage: delete <key> <address>")
				continue
			}

			err := DeleteKey(ctx, parts[1], parts[2])
			if err != nil {
				fmt.Printf("Delete failed: %v\n", err)
			} else {
				fmt.Printf("Delete request for key '%s' completed\n", parts[1])
			}

		case "getall":
			if len(parts) < 2 {
				fmt.Println("Usage: getall <address>")
				continue
			}

			keyValues, err := GetAllKeyValues(ctx, parts[1])
			if err != nil {
				fmt.Printf("GetAll failed: %v\n", err)
			} else {
				if len(keyValues) == 0 {
					fmt.Println("No key-value pairs found")
				} else {
					fmt.Println("Key-value pairs:")
					for k, v := range keyValues {
						fmt.Printf("  %s -> %s\n", k, v)
					}
				}
			}

		case "dump":
			node.dump()

		case "quit":
			fmt.Println("Exiting...")
			return

		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}
	}
}

func main() {
	// Parse command line flags
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	createPort := createCmd.Int("port", 3410, "Port to listen on")

	joinCmd := flag.NewFlagSet("join", flag.ExitOnError)
	joinPort := joinCmd.Int("port", 3410, "Port to listen on")
	joinAddr := joinCmd.String("addr", "", "Address of existing node")

	if len(os.Args) < 2 {
		fmt.Println("Expected 'create' or 'join' subcommand")
		os.Exit(1)
	}

	var node *Node
	var address string

	switch os.Args[1] {
	case "create":
		err := createCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatal(err)
		}

		address = fmt.Sprintf(":%d", *createPort)
		node, err = StartServer(address, "")
		if err != nil {
			log.Fatalf("Failed to create node: %v", err)
		}
		log.Printf("Created new ring with node at %s", node.Address)

	case "join":
		err := joinCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatal(err)
		}

		if *joinAddr == "" {
			log.Fatal("Join requires an address of an existing node")
		}

		address = fmt.Sprintf(":%d", *joinPort)
		node, err = StartServer(address, *joinAddr)
		if err != nil {
			log.Fatalf("Failed to join ring: %v", err)
		}
		log.Printf("Joined ring with node at %s", node.Address)

	default:
		fmt.Println("Expected 'create' or 'join' subcommand")
		os.Exit(1)
	}

	// Run the interactive shell
	RunShell(node)
}
