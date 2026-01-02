# Raft implement

## Part 1: Basic features

### 1. Leader election

Raft algorithm allows servers to elect a leader and only the leader is capable of receiving instruction from client, then passes that command to all other servers (appending log).
As stated in the paper, each server should be one of the three state: leader, candidate or follower. This is the declaration of the `Raft` struct:
```Go
// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// common properties on all servers
	currentTerm int
	votedFor    int
	log         []LogEntry

	// violatile state (lost when killed)
	commitIndex int
	lastApplied int

	// state on leader
	nextIndex  []int
	matchIndex []int

	// healthcheck and role-related
	state         int
	lastHeartBeat time.Time
	applyCh       chan ApplyMsg
	applyCond     *sync.Cond

	// for leader election
	electionTimeout   time.Duration
	lastBroadcastTime time.Time // fix the minor bug "warning: term changed even though there were no failures"

	// 2D: Snapshot state
	lastIncludedIndex int
	lastIncludedTerm  int
}
```
The server's state was set to integer (better comparison).

One of the most unique trait of Raft is randomized, each server will have a different timeout, which is set by the randomized function. This reduces the chance of split vote in the leader election phase. The `Rand` function of Golang is used to implement this feature:
```Go
// reset timer each time (increase term if follower, reset to become candidate)
func (rf *Raft) resetElectionTimer() {
	rf.lastHeartBeat = time.Now()
	ms := 300 + (rand.Int63() % 150) // [300; 450] ms
	rf.electionTimeout = time.Duration(ms) * time.Millisecond
}
```
In the first task: "Leader election", there are two important function `func (rf *Raft) broadcastHeartBeat()` and `func (rf *Raft) startElection()`. These two functions ensure that if there is a leader in this system, the leader will continuously broadcast the heartbeat (by sending an empty log entry) to all other followers. And if the followers do not hear from the current leader more than `leaderTimeOut`, it will start the election.

Whenever a server recieves a heartbeat from leader, it will reset the `timer` (to not accidentally begin the leader election process), it also checks for the term of the received log, to match with current `Term` of the system. 

### 2. Log consistency

Since the leader will receive any instruction from the client, then passes it to all other servers to apply to its state machine, the log must be consistent across all nodes. The approach is to start another `go routine` along with `leader election` rountine, so that for every moments, a machine will both check for current leader (and change its state to `candidate` if the leader was down), also check for any upcoming log and must be consistent even if the leader was down, the recover again. As stated in the figure 2 from the raft paper.

The conflict is resolved inside the function `AppendingEntries`:
```Go
func (rf *Raft) AppendEntries(args *AppendingEntriesArgs, reply *AppendingEntriesReply) {
	// lock first, protect Raft server from itself (other processes)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()

	// false if incoming term is less than us
	// Not accept if sending < self
	...(logic check)
	// false if term is different
	// if args.Entries[args.PrevLogIndex].Term
	...(logic check)

	// sending log ~ heartbeat
	rf.lastHeartBeat = time.Now()

	if rf.state == StateCandidate {
		rf.state = StateFollower // force to become a follower even has larger term
	}
	reply.Term = rf.currentTerm
	// reply.Success = true

	// log consistent 2B
	// 5.3 reply fail if log does not contain any entry at prevLogIndex with term
	// matches to prevLogTerm

	// Case A: PrevLogIndex is behind our snapshot (very rare/outdated RPC)
	if args.PrevLogIndex < rf.lastIncludedIndex {
		reply.Success = false
		reply.ConflictTerm = -1
		reply.ConflictIndex = rf.lastIncludedIndex + 1
		return
	}

	// Case B: PrevLogIndex is beyond our log length
	if args.PrevLogIndex > rf.getLastLogIndex() {
		reply.Success = false
		reply.ConflictIndex = rf.getLastLogIndex() + 1
		reply.ConflictTerm = -1
		return
	}

	// Case C: Term mismatch at PrevLogIndex
	// We can safely access log because we checked PrevLogIndex >= lastIncludedIndex
	if rf.getLogTerm(args.PrevLogIndex) != args.PrevLogTerm {
		// match index but not term (stale data?)
		reply.Success = false
		// reply with the conflict term (and included index)
		reply.ConflictTerm = rf.getLogTerm(args.PrevLogIndex)

		// Find the first index of this conflict term (optimization)
		// Scan backwards, stopping at lastIncludedIndex
		index := args.PrevLogIndex
		for index > rf.lastIncludedIndex && rf.getLogTerm(index) == reply.ConflictTerm {
			index--
		}
		reply.ConflictIndex = index + 1
		return
	}

	reply.Success = true

	// if reply success, append log entries that are missing
	for i, entry := range args.Entries {
		// Logical index of this new entry
		index := args.PrevLogIndex + 1 + i

		// If index is within our existing log
		if index <= rf.getLastLogIndex() {
			if rf.getLogTerm(index) != entry.Term {
				// Conflict: delete everything from here
				// Physical deletion
				physIndex := index - rf.lastIncludedIndex
				if rf.getLogTerm(index) != entry.Term {
					rf.log = rf.log[:physIndex]
					rf.log = append(rf.log, args.Entries[i:]...)
					break
				}
			}
		} else {
			// Append new entries
			rf.log = append(rf.log, entry)
		}
	}

	// if leader commit index is larger than us, set the commit index
	// to min of leader commit and the upcoming commit index
	if args.LeaderCommit > rf.commitIndex {
		lastNewIndex := args.PrevLogIndex + len(args.Entries)
		if args.LeaderCommit < lastNewIndex {
			rf.commitIndex = args.LeaderCommit
		} else {
			rf.commitIndex = lastNewIndex
		}
		//TODO: broadcast the message through the channel
		rf.applyCond.Broadcast()
	}
}
```

This function handle the log conflict in the Raft system, even if the leader experienced multiple downtime events. It has been modified to adpat with the feature 2C and 2D (use snapshot).

## Part 2: Advanced features
### 2.1. Persitent state

Raft server must be able to continue on where it left (server shutdown, outage,...). So the important properties should be captured before each ciritcal process.

```Go
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)

	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)

	rfState := w.Bytes()
	rf.persister.Save(rfState, rf.persister.ReadSnapshot())
}
```

The important properties are: `currentTerm` - indicates the term (time period, how many times the leader election has happened), `votedFor` - the id of the server that this node voted for, `log` - all log entries (till the time this function runs), `lastIncludedIndex` - last index that is in log entry and `lastIncludedTerm`.

The reading, should pass the value to temporary variables first, perform check then passed them to the real fields, this will avoid the error caused by null value.

```Go
	var currentTerm int
	var votedFor int
	var logs []LogEntry
	var lastIncludedIndex int
	var lastIncludedTerm int

	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&logs) != nil ||
		d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&lastIncludedTerm) != nil {
		// DPrintf("Error decoding persistent state")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = logs
		rf.lastIncludedIndex = lastIncludedIndex
		rf.lastIncludedTerm = lastIncludedTerm
	}
```