# About this project

This project is arranged into 3 directories:
- `src` contains the inspiration from [Utah Chrord's example](https://cs.utahtech.edu/cs/3410/asst_chord.html).
- `work` is the group work - main contents for submission.
- `submit` contains some personal work, but is obsolete due to `work` completion.

## Part 1. Basic functions of Chord.

From paper [Berkeley's Chord](https://people.eecs.berkeley.edu/~istoica/papers/2003/chord-ton.pdf), there are several "backbone" functions were implemented to ensure the 'Chord' features.

### 1.1 Find closet preceding node of an `id`.

This is the basic lookup feature. The function allows the chord to find the closest predecessor of an `id`. Example, we need to find an id `x` in the chord ring, this function will return the name (address) of the chord that comes before this `id` in the chord ring (in clockwise).

```Go
// find closet predecessor logic
// check the finger table, if an entry i is between n and id, return that entry
// else retrun n itself
func (n *Node) findClosetPredecessor(id *big.Int) string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var currentID *big.Int
	if n.ID != nil {
		currentID = n.ID
	} else {
		currentID = hash(n.Address)
	}

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
```
To ensure this process happens correctly, use the `mutex` of the chord ring to protect the shared data (entries in the fingertable). The skeletal code provided `between` function to check if a given id lies between [first; start] (inclusive defined by a boolean variable). As in the code, we check if that the address (of entry i in the fingertable) satifies: start-id (lookup node) - current entry - id.

### 1.2. Find successor.

As in the chord, an `id` can only be stored in the first successor of its. Which means, if the can find the closet predecessor of that `id`, we just take the first successor of newly found node -> allocate the `id` to that node.

```Go
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
```

The approach method of `find_successor` is "iterative". So that everytime a node has information about "next" lookup node, it will notify the original asking node and that node will continue asking the next on the list. This approach can avoid the case of "node failure" as in the "recursive" approach. For example, a node A asked node B about successor of ID x, then B tried to ask C, since B is not in charge of storing x,... in this scenario, A must wait for all asking steps to be completed, and if any node in this `chain` was down, maybe A will never know the location of x.

### 1.3. Chord's self-stabilization

Chord ring has an ability to re-distribite the contents during a node joing or node failure event. All nodes in this ring can also reach each others and fix the "broke connection". There are 3 functions to satify these constrains: `stabilize`, `notify` and `fixfinger`.

- `stabilize` allows node to verify its status with n-immediate succesors (each node has a list of n-succesors). By calling this function, node can update the successor list (if any successor fails) or even check and update the predecessor.
- `notify` is called when a node find its potenital successor. As above, when a node A has 3 successors B, C and D. If B failed, A will call `notify` to inform C that "now I'm your predecessor`, C after discarding B as its predecessor, will receive this notification and adopts A as its predecessor.
-`fixfinger` updates the finger table if needed, keeps the finger table always up-to-date.

## Part 2. Advanced features

### 2.3. Fault-tolerance

If a file was uploaded to the chord, it will remain even its owner node is down. Every time a file `x` is uploaded to chord ring, the function `findSuccessor` will determine the location of that file. But to ensure the file won't be lost if that node goes down, it will also replicate that file to all its successors.

```Go
// Put implements the Put RPC method
// adding additional check for replica data accros successors
func (n *Node) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	n.mu.Lock()
	if n.Bucket == nil {
		n.Bucket = make(map[string]string)
	}
	n.Bucket[req.Key] = req.Value
	n.mu.Unlock()

	if req.IsReplica {
		return &pb.PutResponse{}, nil
	}

    // run a go routine to avoid blocking
	go n.replicateToSuccessors(req.Key, req.Value)

	return &pb.PutResponse{}, nil
}
```
And to provide useful information within `PrintState` function, when executing this command, it will show all the status of the files stored in this node (REPLICA or PRIMARY). And only the node with PRIMARY status can see the filename, otherwise, it will only see the hash number.

```Go
// Get a successor of a node (indicated by address) (inside RPC.go)
// Put data to replica
func CallPutReplica(address, key, value string) error {
	address = resolveAddress(address)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	_, err = client.Put(context.Background(), &pb.PutRequest{
		Key:       key,
		Value:     value,
		IsReplica: true,
	})
	return err
}

// helper function to replicate data accros nodes (inside chord.go)
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
```