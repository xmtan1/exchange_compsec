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
	defaultPort       = "3410"
	successorListSize = 3
	keySize           = sha1.Size * 8
	maxLookupSteps    = 32
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
	ID          big.Int
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
func jump(address string, fingerentry int) *big.Int {
	n := hash(address)

	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1)
	distance := new(big.Int).Exp(two, fingerentryminus1, nil)

	sum := new(big.Int).Add(n, distance)

	return new(big.Int).Mod(sum, hashMod)
}

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
	log.Print("ping: received request")
	return &pb.PingResponse{}, nil
}

// Put implements the Put RPC method
func (n *Node) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	log.Print("put: [", req.Key, "] => [", req.Value, "]")
	n.Bucket[req.Key] = req.Value
	return &pb.PutResponse{}, nil
}

// Get implements the Get RPC method
func (n *Node) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	value, exists := n.Bucket[req.Key]
	if !exists {
		log.Print("get: [", req.Key, "] miss")
		return &pb.GetResponse{Value: ""}, nil
	}
	log.Print("get: [", req.Key, "] found [", value, "]")
	return &pb.GetResponse{Value: value}, nil
}

// Delete implements the Delete RPC method
func (n *Node) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.Bucket[req.Key]; exists {
		log.Print("delete: found and deleted [", req.Key, "]")
		delete(n.Bucket, req.Key)
	} else {
		log.Print("delete: not found [", req.Key, "]")
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

// Find the closest (clockwise) predecessor node for an ID, this is called function.
func (n *Node) FindClosestPreceding(ctx context.Context, req *pb.FindClosestPrecedingRequest) (*pb.FindClosestPrecedingResponse, error) {
	n.mu.Lock()
	defer n.mu.RUnlock()

	targetID := new(big.Int)
	targetID.SetString(req.Id, 10)

	closestAddr := n.findClosetPredecessor(targetID)
	return &pb.FindClosestPrecedingResponse{
		Address: closestAddr,
	}, nil
}

// Called by another node
func (n *Node) FindSuccessor(ctx context.Context, req *pb.FindSuccessorRequest) (*pb.FindSuccessorResponse, error) {
	id := new(big.Int)
	_, ok := id.SetString(req.Id, 10)
	if !ok {
		return nil, fmt.Errorf("invalid ID format")
	}
	successorAddr, err := n.findSuccessor(id)
	if err != nil {
		return nil, err
	}
	return &pb.FindSuccessorResponse{
		Address: successorAddr,
	}, nil
}

// Get a list of successors first (successors of a node, not a key)
func (n *Node) GetSuccessorList(ctx context.Context, rep *pb.GetSuccessorListRequest) (*pb.GetSuccessorListResponse, error) {
	n.mu.Lock()
	defer n.mu.RUnlock()

	return &pb.GetSuccessorListResponse{
		Successors: n.Successors,
	}, nil
}

// Find the successor of id, performed by node n
// try to jump to closet predecessor of that id
// this is the logic code
func (n *Node) findSuccessor(id *big.Int) (string, error) {
	currentCandidate := n.findClosetPredecessor(id)

	// if the node n itself is the closet predecessor of that id ( n - id - first successor of n)
	if currentCandidate == n.Address {
		if len(n.Successors) == 0 {
			return "", fmt.Errorf("node has no succesor")
		}
		return n.Successors[0], nil
	}

	steps := 0

	for {
		if steps > maxLookupSteps {
			return "", fmt.Errorf("lookup failed, steps exceeded, allow %d", maxLookupSteps)
		}
		steps++
		// Iterative lookup
		// get the first successor of the current candidate first
		// (closet predecessor of id - id - first successor of closet predecessor of id)
		candidateSuccessor, _ := GetSuccessorList(currentCandidate)

		if len(candidateSuccessor) == 0 {
			return "", fmt.Errorf("node %s has empty successor list", currentCandidate)
		}

		candID := hash(currentCandidate)
		succID := hash(candidateSuccessor[0])

		if between(candID, id, succID, true) {
			return candidateSuccessor[0], nil
		}

		// if not, jump to next closet predecessor
		nextHop, err := FindClosetPredecessor(context.Background(), currentCandidate, id.String())
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
	n.mu.Lock()
	defer n.mu.RUnlock()

	currentID := hash(n.Address)

	for i := keySize; i >= 1; i-- {
		fingerAddr := n.FingerTable[i]
		// skip the null entry
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
	predAddr := n.Predecessor
	n.mu.RUnlock()

	if predAddr == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := PingNode(ctx, predAddr)

	if err != nil {
		log.Printf("Predecessor %s is dead. Clearing.", predAddr)
		n.mu.Lock() // Lock for writing
		n.Predecessor = ""
		n.mu.Unlock()
	}
}

func (n *Node) Notify(ctx context.Context, req *pb.NotifyRequest) (*pb.NotifyResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	predNode := req.Address

	if n.Predecessor == "" {
		n.Predecessor = predNode
		return &pb.NotifyResponse{}, nil
	}

	hashID := hash(n.Address)
	currentPredNodeID := hash(n.Predecessor)
	predNodeID := hash(predNode)

	if between(currentPredNodeID, predNodeID, hashID, false) {
		n.Predecessor = predNode
	}

	return &pb.NotifyResponse{}, nil
}

func (n *Node) GetPredecessor(ctx context.Context, req *pb.GetPredecessorRequest) (*pb.GetPredecessorResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.Predecessor == "" {
		return &pb.GetPredecessorResponse{
			Address: "",
		}, nil
	}

	return &pb.GetPredecessorResponse{
		Address: n.Predecessor,
	}, nil
}

// Internal self function to stabilize after fault of successors
// to keep things simple, maintain a fixed of successor list (const)
// or it can be use when a new node join in between
// n - x - first successor
func (n *Node) stabilize() {
	// TODO: Student will implement this
	n.mu.RLock()
	firstSuccessor := n.Successors[0]
	n.mu.RUnlock()

	x, err := GetPredecessor(firstSuccessor)

	if err == nil && x != "" {
		if between(hash(n.Address), hash(x), hash(firstSuccessor), false) {
			n.mu.Lock()
			n.Successors[0] = x
			n.mu.Unlock()

			firstSuccessor = x
		}
	}

	Notify(firstSuccessor, n.Address)

	successorList, err := GetSuccessorList(firstSuccessor)
	if err == nil {
		n.mu.Lock()
		defer n.mu.Unlock()

		newList := []string{firstSuccessor}

		newList = append(newList, successorList...)

		if len(newList) > successorListSize {
			newList = newList[:successorListSize]
		}

		n.Successors = newList
	}
}

func (n *Node) fixFingers(nextFinger int) int {
	// TODO: Student will implement this
	nextFinger++
	if nextFinger > keySize {
		nextFinger = 1
	}
	nextID := jump(n.Address, nextFinger)

	succAddr, err := n.findSuccessor(nextID)

	if err == nil {
		n.mu.Lock()
		n.FingerTable[nextFinger] = succAddr
		n.mu.Unlock()
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
	fmt.Println("Dump: information about this node")

	// predecessor and successor links
	fmt.Println("Neighborhood")
	fmt.Println("pred:   ", addr(n.Predecessor))
	fmt.Println("self:   ", addr(n.Address))
	for i, succ := range n.Successors {
		fmt.Printf("succ  %d: %s\n", i, addr(succ))
	}
	fmt.Println()
	fmt.Println("Finger table")
	i := 1
	for i <= keySize {
		for i < keySize && n.FingerTable[i] == n.FingerTable[i+1] {
			i++
		}
		fmt.Printf(" [%3d]: %s\n", i, addr(n.FingerTable[i]))
		i++
	}
	fmt.Println()
	fmt.Println("Data items")
	for k, v := range n.Bucket {
		s := fmt.Sprintf("%040x", hash(k))
		fmt.Printf("    %s.. %s => %s\n", s[:8], k, v)
	}
	fmt.Println()
}
