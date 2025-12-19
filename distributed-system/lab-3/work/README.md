# Distributed Chord DHT System

## 📂 Project Structure

This project is organized into three main directories:

- `src/`: Contains reference implementations inspired by Utah Chord's example.
- `work/`: The main group submission containing the core source code and Dockerfiles.
- `submit/`: Contains preliminary personal work (superseded by `work`).

---

## Part 1: Basic Functions of Chord

Based on the Berkeley Chord Paper[Berkeley's Chord](https://people.eecs.berkeley.edu/~istoica/papers/2003/chord-ton.pdf), we implemented several "backbone" functions to ensure correct Distributed Hash Table (DHT) behavior.

### 1.1 Find closest preceding node of an `id`.

This is the basic lookup feature. The function allows the chord to find the closest predecessor of an `id`. Example, we need to find an id `x` in the chord ring, this function will return the name (address) of the chord that comes before this `id` in the chord ring (in clockwise).

```Go
// find closest predecessor logic
// check the finger table, if an entry i is between n and id, return that entry
// else return n itself
func (n *Node) findclosestPredecessor(id *big.Int) string {
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

As in the chord, an `id` can only be stored in the first successor of its. Which means, if the can find the closest predecessor of that `id`, we just take the first successor of newly found node -> allocate the `id` to that node.

```Go
// Find the successor of id, performed by node n
// try to jump to closest predecessor of that id
// this is the logic code
func (n *Node) findSuccessor(id *big.Int) (string, error) {
	currentCandidate := n.findclosestPredecessor(id)

	// if the node n itself is the closest predecessor of that id ( n - id - first successor of n)
	if currentCandidate == n.Address {
		if len(n.Successors) == 0 {
			return "", fmt.Errorf("[ERROR] Node has no succesorss")
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
		// (closest predecessor of id - id - first successor of closest predecessor of id)
		candidateSuccessor, _ := CallGetSuccessorList(currentCandidate)

		if len(candidateSuccessor) == 0 {
			return "", fmt.Errorf("[ERROR] Node %s has empty successor list", currentCandidate)
		}

		candID := hash(currentCandidate)
		succID := hash(candidateSuccessor[0])

		if between(candID, id, succID, true) {
			return candidateSuccessor[0], nil
		}

		// if not, jump to next closest predecessor
		nextHop, err := CallFindclosestPredecessor(context.Background(), currentCandidate, id.String())
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

**Why Iterative?** We chose an iterative approach over recursive. If a node in the chain fails during a recursive call, the original requester might hang indefinitely. In the iterative approach, the requester controls the process and can handle timeouts or failures more gracefully.

### 1.3 Self-Stabilization

The Chord ring must handle dynamic joins and failures. Three key functions maintain ring integrity:

- **Stabilize**: Verifies the node's immediate successor and updates the list.
- **Notify**: Informs a node of its potential new predecessor.
- **FixFinger**: Periodically updates the Finger Table to ensure efficient lookups.

---

## Part 2: Advanced Features

### 2.1 Data Security (Encryption)

Files are never stored in plain text. We implemented **AES-GCM (Advanced Encryption Standard with Galois/Counter Mode)** for end-to-end encryption.

- **Encryption**: The client encrypts the file content using a shared secret key before RPC transmission.
- **Storage**: Nodes store only encrypted blobs. Even if a bucket is inspected, the data is unreadable without the key.
- **Decryption**: The client decrypts the data upon retrieval (**Lookup**).

### 2.2 Transport Security (TLS)

To prevent Man-in-the-Middle (MITM) attacks, all communication is secured via TLS.

- **Certificates**: Nodes load `server.crt` and `server.key` on startup.
- **Secure gRPC**: We replaced `insecure.NewCredentials()` with `credentials.NewTLS(...)`. All traffic (Stabilize, Notify, Put, Get) is encrypted.

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
In the `Lookup` command, if the client detects that the PRIMARY owner is down (connection error) or the data is missing, it triggers a recovery mechanism:

1. The client calculates Hash(PrimaryID + 1).

2. It queries the ring to find the successor of this new ID (which corresponds to the first REPLICA node).

3. It attempts to retrieve the file from the replica. This ensures that as long as one replica survives, the user can still retrieve and decrypt the file.


## Part 3: Automated Testing & Cloud Deployment

### 3.1 Automated Testing

We developed a Python orchestrator (`test_scenario.py`) to verify encryption and fault tolerance without manual intervention.

**The Test Scenario:**

1. **Bootstrap**: Starts 3 local Chord nodes (Ports `4170`, `4171`, `4172`).
2. **Secure Storage**: Client encrypts and stores `secret.txt`.
3. **Chaos**: The script kills the Primary Node (`4170`).
4. **Recovery**: The client attempts to retrieve the file.
5. **Verification**: The client must locate the Replica (`4171` or `4172`) and successfully decrypt the data.

**How to Run:**

```bash
python3 ./test_scenario.py
```

### 3.2 Cloud Deployment (AWS EC2)

This section details deploying the Chord network across multiple AWS EC2 instances using Docker in **Host Network** mode.

#### 📋 Prerequisites

- **2 AWS EC2 Instances** (Amazon Linux 2023).
- **Security Groups**: Allow TCP traffic on ports `3000-3010`.

#### 🚀 Step 1: Environment Setup (All Nodes)

Run these commands on both EC2 instances.

```bash
# Connect to EC2
ssh -i "labsuser.pem" ec2-user@<Public_IP>

# Install Docker & Git
sudo yum update -y && sudo yum install git -y
sudo systemctl start docker && sudo systemctl enable docker

# Clone & Build
git clone https://github.com/TurlingXian/devops-docs.git
cd ~/work
sudo docker build -t chord-node .
```

#### 🛡️ Step 2: AWS Networking

Ensure the Security Group attached to your instances allows Inbound **Custom TCP** traffic on ports `3000-3010` from Anywhere (`0.0.0.0/0`) or the other instance's **Private IP**.

#### 🌐 Step 3: Deployment (2-Node Setup)

**On EC2 Instance A (Node 1 - Bootstrap):**

```bash
# Replace <Node1_Private_IP> with EC2 A's actual Private IP
sudo docker run -d -it \
  --name node1 \
  --network host \
  chord-node \
  -a <Node1_Private_IP> \
  -p 3000 \
  --ts 3000 --tff 1000 --tcp 3000 -r 4
```

**On EC2 Instance B (Node 2 - Joiner):**

```bash
# Replace <Node2_Private_IP> with EC2 B's IP
# Replace <Node1_Private_IP> with EC2 A's IP
sudo docker run -d -it \
  --name node2 \
  --network host \
  chord-node \
  -a <Node2_Private_IP> \
  -p 3000 \
  --ja <Node1_Private_IP> \
  --jp 3000 \
  --ts 3000 --tff 1000 --tcp 3000 -r 4
```

#### ✅ Step 4: Verification (Cross-Instance Test)

**Store on Node 1:**

```bash
sudo docker exec node1 sh -c "echo 'Cross Instance Data' > secret.txt"
sudo docker attach node1
> storefile secret.txt
# Press Ctrl+P, Ctrl+Q to exit
```

**Retrieve on Node 2:**

```bash
sudo docker attach node2
> lookup secret.txt
```

**Success Criteria:** The file content is displayed despite being stored on a different physical machine.

---

### 📺 3.3 Scaling to 8 Nodes (Hybrid Deployment)

To demonstrate a robust ring, we deploy 8 nodes split across the two instances (4 nodes per EC2).

#### 🟢 EC2 Instance A (Leader & Nodes 3, 4, 5)

Use `tmux` to split your terminal if desired.

```bash
# Node 1 (Bootstrap) - Clean up old container first
sudo docker rm -f node1
sudo docker run -d -it --name node1 --network host chord-node -a <IP_A> -p 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4

# Node 3
sudo docker run -d -it --name node3 --network host chord-node -a <IP_A> -p 3001 --ja <IP_A> --jp 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4

# Node 4
sudo docker run -d -it --name node4 --network host chord-node -a <IP_A> -p 3002 --ja <IP_A> --jp 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4

# Node 5
sudo docker run -d -it --name node5 --network host chord-node -a <IP_A> -p 3003 --ja <IP_A> --jp 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4
```

#### 🔵 EC2 Instance B (Joiners 2, 6, 7, 8)

```bash
# Node 2
sudo docker run -d -it --name node2 --network host chord-node -a <IP_B> -p 3000 --ja <IP_A> --jp 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4

# Node 6
sudo docker run -d -it --name node6 --network host chord-node -a <IP_B> -p 3001 --ja <IP_A> --jp 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4

# Node 7
sudo docker run -d -it --name node7 --network host chord-node -a <IP_B> -p 3002 --ja <IP_A> --jp 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4

# Node 8
sudo docker run -d -it --name node8 --network host chord-node -a <IP_B> -p 3003 --ja <IP_A> --jp 3000 --ts 3000 --tff 1000 --tcp 3000 -r 4
```

#### 📸 Verification

To verify all 8 nodes are active:

1. Open two terminal windows side-by-side.
2. Run `sudo docker ps` in both.
3. You should see 4 containers running on each machine.
