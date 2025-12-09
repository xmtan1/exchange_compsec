package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	pb "chord/protocol" // Update path as needed
)

const (
	defaultPort = "3410"
	// successorListSize = 3
	keySize        = sha1.Size * 8
	maxLookupSteps = 32
)

var (
	two     = big.NewInt(2)
	hashMod = new(big.Int).Exp(big.NewInt(2), big.NewInt(keySize), nil)
)

// Node represents a node in the Chord DHT
type Node struct {
	pb.UnimplementedChordServer
	mu sync.RWMutex

	Address     string
	ID          *big.Int
	Predecessor string
	Successors  []string
	FingerTable []string

	Bucket map[string]string

	TimeStabilize      time.Duration
	TimeFixFinger      time.Duration
	TimeCheckPred      time.Duration
	NumberOfSuccessors int
}

// get the sha1 hash of a string as a bigint
func hash(elt string) *big.Int {
	hasher := sha1.New()
	hasher.Write([]byte(elt))
	return new(big.Int).SetBytes(hasher.Sum(nil))
}

// calculate the address of a point somewhere across the ring
// this gets the target point for a given finger table entry
// the successor of this point is the finger table entry
// func jump(address string, fingerentry int) *big.Int {
// 	n := hash(address)

// 	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1)
// 	distance := new(big.Int).Exp(two, fingerentryminus1, nil)

// 	sum := new(big.Int).Add(n, distance)

// 	return new(big.Int).Mod(sum, hashMod)
// }

// returns true if elt is between start and end, accounting for the right
// if inclusive is true, it can match the end
func between(start, elt, end *big.Int, inclusive bool) bool {
	if end.Cmp(start) > 0 {
		return (start.Cmp(elt) < 0 && elt.Cmp(end) < 0) || (inclusive && elt.Cmp(end) == 0)
	} else {
		return start.Cmp(elt) < 0 || elt.Cmp(end) < 0 || (inclusive && elt.Cmp(end) == 0)
	}
}

// Ping implements the Ping RPC method
func (n *Node) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	// log.Print("ping: received request")
	return &pb.PingResponse{}, nil
}

// Put implements the Put RPC method
// adding additional check for replica data accros successors
func (n *Node) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.Bucket == nil {
		n.Bucket = make(map[string]string)
	}
	n.Bucket[req.Key] = req.Value

	if req.IsReplica {
		log.Printf("[INFO] Store REPLICA for key %s", req.Key)
	} else {
		log.Printf("[INFO] Store PRIMARY for key %s", req.Key)
	}

	if !req.IsReplica {
		go n.replicateToSuccessors(req.Key, req.Value)
	}

	return &pb.PutResponse{}, nil
}

// Get implements the Get RPC method
func (n *Node) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	value, exists := n.Bucket[req.Key]
	if !exists {
		log.Print("[WARN] Get: [", req.Key, "] miss")
		return &pb.GetResponse{Value: ""}, nil
	}
	log.Print("[INFO] Get: [", req.Key, "] found [", value, "]")
	return &pb.GetResponse{Value: value}, nil
}

// Delete implements the Delete RPC method
func (n *Node) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.Bucket[req.Key]; exists {
		log.Print("[LOG] Delete: found and deleted [", req.Key, "]")
		delete(n.Bucket, req.Key)
	} else {
		log.Print("[WARN] Delete: not found [", req.Key, "]")
	}
	return &pb.DeleteResponse{}, nil
}

// GetAll implements the GetAll RPC method
func (n *Node) GetAll(ctx context.Context, req *pb.GetAllRequest) (*pb.GetAllResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	log.Printf("getall: returning %d key-value pairs", len(n.Bucket))

	// Create a copy of the bucket map
	keyValues := make(map[string]string)
	for k, v := range n.Bucket {
		keyValues[k] = v
	}

	return &pb.GetAllResponse{KeyValues: keyValues}, nil
}

// helper function to replicate data accros nodes
func (n *Node) replicateToSuccessors(key, value string) {
	n.mu.RLock()
	successors := make([]string, len(n.Successors))
	// a list of destinations
	copy(successors, n.Successors)
	n.mu.RUnlock()

	// release the lock for other operation
	replicationNumber := n.NumberOfSuccessors

	for i := 0; i < replicationNumber; i++ {
		addr := n.Successors[i]
		if addr == n.Address {
			continue
		}
		err := CallPutReplica(addr, key, value)
		if err != nil {
			log.Printf("[ERROR] Falied to replica key %s to address %s: %v", key, addr, err)
		}
	}
}

// Find the successor of id, performed by node n
// try to jump to closet predecessor of that id
// this is the logic code
func (n *Node) findSuccessor(id *big.Int) (string, error) {
	currentCandidate := n.findClosetPredecessor(id)

	// if the node n itself is the closet predecessor of that id ( n - id - first successor of n)
	if currentCandidate == n.Address {
		if len(n.Successors) == 0 {
			return "", fmt.Errorf("[ERROR] Node has no succesor")
		}
		return n.Successors[0], nil
	}

	steps := 0

	for {
		if steps > maxLookupSteps {
			return "", fmt.Errorf("[ERROR] Lookup failed, steps exceeded, allow %d", maxLookupSteps)
		}
		steps++
		// Iterative lookup
		// get the first successor of the current candidate first
		// (closet predecessor of id - id - first successor of closet predecessor of id)
		candidateSuccessor, _ := CallGetSuccessorList(currentCandidate)

		if len(candidateSuccessor) == 0 {
			return "", fmt.Errorf("[ERROR] Node %s has empty successor list", currentCandidate)
		}

		candID := hash(currentCandidate)
		succID := hash(candidateSuccessor[0])

		if between(candID, id, succID, true) {
			return candidateSuccessor[0], nil
		}

		// if not, jump to next closet predecessor
		nextHop, err := CallFindClosetPredecessor(context.Background(), currentCandidate, id.String())
		if err != nil {
			return "", err
		}

		if nextHop == currentCandidate {
			// If we are stuck on a node, return its successor as the best guess (maybe there is some link failure?)
			return candidateSuccessor[0], nil
		}

		currentCandidate = nextHop
	}
}

// find closet predecessor logic
// check the finger table, if an entry i is between n and id, return that entry
// else retrun n itself
func (n *Node) findClosetPredecessor(id *big.Int) string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	currentID := hash(n.Address)

	for i := keySize; i >= 1; i-- {
		fingerAddr := n.FingerTable[i]
		if fingerAddr == "" {
			continue
		}

		fingerID := hash(fingerAddr)

		if between(currentID, fingerID, id, false) {
			return fingerAddr
		}
	}
	return n.Address
}

// Check predecessor logic, check for alive one
func (n *Node) checkPredecessor() {
	n.mu.RLock()
	pred := n.Predecessor
	n.mu.RUnlock()

	if pred == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := PingNode(ctx, pred); err != nil {
		log.Printf("[INFO] CheckPredecessor: %s seems dead: %v, clearing predecessor", pred, err)
		n.mu.Lock()
		if n.Predecessor == pred {
			n.Predecessor = ""
		}
		n.mu.Unlock()
	}
}

// Internal self function to stabilize after fault of successors
// to keep things simple, maintain a fixed of successor list (const)
// or it can be use when a new node join in between
// n - x - first successor
func (n *Node) stabilize() {
	n.mu.RLock()
	if len(n.Successors) == 0 {
		n.mu.RUnlock()
		return
	}
	succList := make([]string, len(n.Successors))
	copy(succList, n.Successors)

	n.mu.RUnlock()

	// check first successor, skip dead node
	var succAddr string
	var succIndex int
	foundAlive := false

	for i, s := range succList {
		_, err := CallGetPredecessor(s)
		if err == nil {
			succAddr = s
			succIndex = i
			foundAlive = true
			break
		}
	}

	// if no successor alive -> only one in this ring
	if !foundAlive {
		return
	}

	// if found someone but itself, because succ[0] == itself
	if succIndex > 0 {
		n.mu.Lock()
		if succIndex < len(n.Successors) {
			n.Successors = n.Successors[succIndex:]
		}
	}

	x, err := CallGetPredecessor(succAddr)
	if err == nil && x != "" {
		nID := n.ID
		succID := hash(succAddr)
		xID := hash(x)

		if between(nID, xID, succID, false) {
			succAddr = x
		}
	}

	_ = CallNotify(succAddr, n.Address)

	remoteList, err := CallGetSuccessorList(succAddr)
	if err == nil {
		newList := append([]string{succAddr}, remoteList...)
		if len(newList) > n.NumberOfSuccessors {
			newList = newList[:n.NumberOfSuccessors]
		}
		n.mu.Lock()
		n.Successors = newList
		n.mu.Unlock()
	}
}

func (n *Node) fixFingers(nextFinger int) int {
	if nextFinger < 1 || nextFinger > keySize {
		nextFinger = 1
	}

	// ID：n.ID + 2^(i-1)
	n.mu.RLock()
	selfID := new(big.Int).Set(n.ID)
	n.mu.RUnlock()

	offset := new(big.Int).Exp(two, big.NewInt(int64(nextFinger-1)), nil)
	target := new(big.Int).Add(selfID, offset)
	target.Mod(target, hashMod)

	// target := jump(n.Address, nextFinger)

	addr, err := n.findSuccessor(target)
	if err == nil && addr != "" {
		n.mu.Lock()
		n.FingerTable[nextFinger] = addr
		n.mu.Unlock()
	}

	nextFinger++
	if nextFinger > keySize {
		nextFinger = 1
	}
	return nextFinger
}

// format an address for printing
func addr(a string) string {
	if a == "" {
		return "(empty)"
	}
	s := fmt.Sprintf("%040x", hash(a))
	return s[:8] + ".. (" + a + ")"
}

// print useful info about the local node
func (n *Node) dump() {
	n.mu.RLock()
	defer n.mu.RUnlock()

	fmt.Println()
	fmt.Println("[INFO] Dump: information about this node")

	// Predecessor and Successor links
	fmt.Println("Neighborhood")
	fmt.Println("pred:   ", addr(n.Predecessor))
	fmt.Println("self:   ", addr(n.Address))
	for i, succ := range n.Successors {
		fmt.Printf("succ  %d: %s\n", i, addr(succ))
	}

	fmt.Println("\nFinger table")
	for i := 1; i <= keySize; i++ {
		val := n.FingerTable[i]

		if val == "" {
			continue
		}

		start := i
		for i < keySize && n.FingerTable[i+1] == val {
			i++
		}
		end := i

		if start == end {
			fmt.Printf(" [%3d]:       %s\n", start, addr(val))
		} else {
			fmt.Printf(" [%3d-%3d]: %s\n", start, end, addr(val))
		}
	}

	// Data Items
	fmt.Println("\nData items")
	if len(n.Bucket) == 0 {
		fmt.Println(" (empty)")
	} else {
		// Prepare IDs for ownership check
		// respects -i flag
		var predID *big.Int
		if n.Predecessor != "" {
			predID = hash(n.Predecessor)
		}

		fmt.Printf(" %-20s | %-10s | %-10s | %s\n", "Filename", "Hash ID", "State", "Size")
		fmt.Println(" ----------------------------------------------------------------")

		for filename, content := range n.Bucket {
			// 1. Calculate the file's Hash ID
			fileID := hash(filename)
			shortID := fmt.Sprintf("%040x", fileID)[:8] + ".."

			state := "REPLICA"

			if n.Predecessor == "" || n.Predecessor == n.Address {
				state = "PRIMARY"
			} else {
				if between(predID, fileID, n.ID, true) {
					state = "PRIMARY"
				}
			}

			fmt.Printf(" %-20s | %-10s | %-10s | %d bytes\n",
				filename, shortID, state, len(content))
		}
	}
	fmt.Println()
}

// FindClosestPreceding implements the RPC: return this node's closest preceding node for id
// handler for RPC call
func (n *Node) FindClosestPreceding(ctx context.Context, req *pb.FindClosestPrecedingRequest) (*pb.FindClosestPrecedingResponse, error) {
	id := new(big.Int)
	// client 那边传的是 id.String()（十进制），所以这里用 base 10
	if _, ok := id.SetString(req.Id, 10); !ok {
		return nil, fmt.Errorf("[ERROR] Invalid id: %s", req.Id)
	}

	addr := n.findClosetPredecessor(id)
	return &pb.FindClosestPrecedingResponse{Address: addr}, nil
}

// FindSuccessor implements the RPC version of findSuccessor
func (n *Node) FindSuccessor(ctx context.Context, req *pb.FindSuccessorRequest) (*pb.FindSuccessorResponse, error) {
	id := new(big.Int)
	if _, ok := id.SetString(req.Id, 10); !ok {
		return nil, fmt.Errorf("[ERROR] Invalid id: %s", req.Id)
	}

	addr, err := n.findSuccessor(id)
	if err != nil {
		return nil, err
	}
	return &pb.FindSuccessorResponse{Address: addr}, nil
}

// GetSuccessorList returns this node's successor list
// handler RCP call
func (n *Node) GetSuccessorList(ctx context.Context, req *pb.GetSuccessorListRequest) (*pb.GetSuccessorListResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	succs := make([]string, len(n.Successors))
	copy(succs, n.Successors)
	return &pb.GetSuccessorListResponse{Successors: succs}, nil
}

// Notify implements Chord / notify(n')
// handler for the RPC call CallNotify
func (n *Node) Notify(ctx context.Context, req *pb.NotifyRequest) (*pb.NotifyResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	cand := req.Address
	if cand == "" {
		return &pb.NotifyResponse{}, nil
	}

	if n.Predecessor == "" {
		n.Predecessor = cand
		return &pb.NotifyResponse{}, nil
	}

	selfID := hash(n.Address)
	predID := hash(n.Predecessor)
	candID := hash(cand)

	if between(predID, candID, selfID, false) {
		n.Predecessor = cand
	}

	return &pb.NotifyResponse{}, nil
}

// GetPredecessor returns this node's predecessor
// handler of RPC call
func (n *Node) GetPredecessor(ctx context.Context, req *pb.GetPredecessorRequest) (*pb.GetPredecessorResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return &pb.GetPredecessorResponse{Address: n.Predecessor}, nil
}
