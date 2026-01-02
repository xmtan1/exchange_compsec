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