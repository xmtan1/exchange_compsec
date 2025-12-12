package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "chord/protocol" // Update path as needed

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
func StartServer(address string, nprime string, ts int, tff int, tcp int, r int, id string) (*Node, error) {
	address = resolveAddress(address)

	var NodeID *big.Int

	if id != "" {
		NodeID = new(big.Int)
		if _, ok := NodeID.SetString(id, 16); !ok {
			return nil, fmt.Errorf("invalid ID format: %s", id)
		}
	} else {
		NodeID = hash(address)
	}

	node := &Node{
		Address:            address,
		ID:                 NodeID,
		FingerTable:        make([]string, keySize+1),
		Predecessor:        "",
		Successors:         nil,
		Bucket:             make(map[string]string),
		TimeStabilize:      time.Duration(ts) * time.Millisecond,
		TimeFixFinger:      time.Duration(tff) * time.Millisecond,
		TimeCheckPred:      time.Duration(tcp) * time.Millisecond,
		NumberOfSuccessors: r,
	}

	// Are we the first node?
	if nprime == "" {
		log.Print("[INFO] StartServer: creating new ring")
		node.Successors = []string{node.Address}
	} else {
		log.Print("[INFO] StartServer: joining existing ring using ", nprime)
		// For now use the given address as our successor
		nprime = resolveAddress(nprime)
		node.Successors = []string{nprime}
		// TODO: use a GetAll request to populate our bucket
		remoteBucket, err := GetAllKeyValues(context.Background(), nprime)
		if err != nil {
			log.Printf("[WARNING]: Failed to fetch data: %v", err)
			node.Bucket = make(map[string]string)
		} else if remoteBucket != nil {
			node.Bucket = remoteBucket
		}

		if node.Bucket == nil {
			node.Bucket = make(map[string]string)
		}
	}

	// loading TLS creds
	creds, err := credentials.NewServerTLSFromFile("server.crt", "server.key")
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS keys: %v", err)
	}

	// Start listening for RPC calls(including creds)
	grpcServer := grpc.NewServer(grpc.Creds(creds))

	pb.RegisterChordServer(grpcServer, node)

	lis, err := net.Listen("tcp", node.Address)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Failed to listen: %v", err)
	}

	// Start server in goroutine
	log.Printf("[INFFO] Starting Chord node server on %s", node.Address)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("[ERROR] Failed to serve: %v", err)
		}
	}()

	// Start background tasks
	go func() {
		tickerStab := time.NewTicker(node.TimeStabilize)
		tickerFix := time.NewTicker(node.TimeFixFinger)
		tickerCheck := time.NewTicker(node.TimeCheckPred)
		defer tickerStab.Stop()
		defer tickerFix.Stop()
		defer tickerCheck.Stop()

		nextFinger := 1

		for {
			select {
			case <-tickerStab.C:
				node.stabilize()
			case <-tickerFix.C:
				nextFinger = node.fixFingers(nextFinger)
			case <-tickerCheck.C:
				node.checkPredecessor()
			}
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
			fmt.Println("  storefile <path>           - Store a local text file into the Chord ring")
			fmt.Println("  lookup <filename>          - Lookup a file in the Chord ring and print its content")
			fmt.Println("  printstate                 - Print this node's Chord state")
			// fmt.Println("  dump              - Display info about the current node")
			fmt.Println("  Quit              - Exit the program")

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

		case "storefile":
			if len(parts) < 2 {
				fmt.Println("Usage: StoreFile <path>")
				continue
			}
			path := parts[1]

			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("StoreFile: failed to read %s: %v\n", path, err)
				continue
			}

			// encrypt
			encryptedValue, err := encrypt(string(data))
			if err != nil {
				fmt.Printf("StoreFile: Encryption failed: %v\n", err)
				continue
			}

			filename := filepath.Base(path)
			id := hash(filename)

			succ, err := node.findSuccessor(id)
			if err != nil {
				fmt.Printf("StoreFile: findSuccessor failed: %v\n", err)
				continue
			}

			if err := PutKeyValue(ctx, filename, encryptedValue, succ); err != nil {
				fmt.Printf("StoreFile: RPC put to %s failed: %v\n", succ, err)
				continue
			}

			fmt.Printf("Stored file %q at node %s\n", filename, succ)

		case "lookup":
			if len(parts) < 2 {
				fmt.Println("Usage: Lookup <filename>")
				continue
			}
			filename := parts[1]
			id := hash(filename)

			// try to find Primary Owner
			targetNode, err := node.findSuccessor(id)
			if err != nil {
				fmt.Printf("[ERROR] Lookup: findSuccessor failed: %v\n", err)
				continue
			}

			fmt.Printf("[INFO] Primary owner found: %s. Attempting retrieval...\n", targetNode)

			value, err := GetValue(ctx, filename, targetNode)

			// Fault Tolerance Read
			if err != nil || value == "" {
				fmt.Printf("[WARN] Primary (%s) failed or miss. Error: %v. Trying replicas...\n", targetNode, err)

				one := big.NewInt(1)
				backupID := new(big.Int).Add(id, one)
				backupNode, err2 := node.findSuccessor(backupID)

				if err2 == nil && backupNode != targetNode {
					fmt.Printf("[INFO] Trying backup node: %s...\n", backupNode)
					value, err = GetValue(ctx, filename, backupNode)
				}

			}

			if value == "" {
				fmt.Printf("[ERROR] File %q not found in ring (checked primary and backup).\n", filename)
				continue
			}

			// decrypt
			decrypted, err := decrypt(value)
			if err != nil {
				fmt.Printf("Decryption failed (maybe key mismatch?): %v\n", err)
				fmt.Println("Raw Content:", value)
				continue
			}

			fmt.Println("File content retrieved successfully:")
			fmt.Println(decrypted)

		case "printstate":
			node.dump()

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

// Helper to validate integer ranges
func validateRange(name string, val, min, max int) {
	if val < min || val > max {
		log.Fatalf("Error: --%s must be between %d and %d (got %d)", name, min, max, val)
	}
}

func main() {
	// Parse command line flags
	address := flag.String("a", "", "IP address to bind the chord")
	port := flag.Int("p", 0, "Port to bind/listen")

	joinAddr := flag.String("ja", "", "Existing address of a chord server for this node to join")
	joinPort := flag.Int("jp", 0, "Existing port of a chord server currently advertising to join")

	ts := flag.Int("ts", 0, "Stabilize interval (ms)")
	tff := flag.Int("tff", 0, "Fix finger interval (ms)")
	tcp := flag.Int("tcp", 0, "Check predecessor interval (ms)")

	r := flag.Int("r", 0, "Number of successors to maintain")

	id := flag.String("i", "", "Manual identifier (SHA1 hex string)")

	flag.Parse()

	if *address == "" {
		log.Fatal("Error: -a (Address) is required")
	}

	if *port == 0 {
		log.Fatal("Error: -p (Port) is required")
	}

	validateRange("ts", *ts, 1, 60000)
	validateRange("tff", *tff, 1, 60000)
	validateRange("tcp", *tcp, 1, 60000)
	validateRange("r", *r, 1, 32)

	if (*joinAddr != "" && *joinPort == 0) || (*joinAddr == "" && *joinPort != 0) {
		log.Fatal("Error: Both --ja and --jp must be specified to join a ring")
	}

	bindAddress := fmt.Sprintf("%s:%d", *address, *port)
	var remoteAddress string

	if *joinAddr != "" {
		remoteAddress = fmt.Sprintf("%s:%d", *joinAddr, *joinPort)
	}

	log.Printf("Starting a chord node at %s", bindAddress)
	if remoteAddress != "" {
		log.Printf("Join an existing chord network at %s", remoteAddress)
	} else {
		log.Printf("Starting a new chord ring...")
	}

	node, err := StartServer(bindAddress, remoteAddress, *ts, *tff, *tcp, *r, *id)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Run the interactive shell
	RunShell(node)
}
