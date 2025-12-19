package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

const (
	StateFollower  = 0
	StateCandidate = 1
	StateLeader    = 2
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

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

	// violatile state
	commitIndex int
	lastApplied int

	// state on leader
	nextIndex  []int
	matchIndex []int

	// healthcheck and role-related
	state         int
	lastHeartBeat time.Time
	applyCh       chan ApplyMsg

	// for leader election
	electionTimeout time.Duration
}

// reset timer each time (increase term if follower, reset to become candidate)\
func (rf *Raft) resetElectionTimer() {
	rf.lastHeartBeat = time.Now()
	ms := 300 + (rand.Int63() % 150) // [300; 450] ms
	rf.electionTimeout = time.Duration(ms) * time.Millisecond
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	return rf.currentTerm, rf.state == StateLeader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// broadcast Heartbeat of the leader
// frequently send this to all follower
func (rf *Raft) broadcastHeartBeat() {
	for i := range rf.peers {
		if i == rf.me {
			continue
		}

		go func(server int) {
			rf.mu.Lock()
			if rf.state != StateLeader {
				rf.mu.Unlock()
				return
			}
			args := AppendingEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: len(rf.log) - 1,
				PrevLogTerm:  rf.log[len(rf.log)-1].Term,
				Entries:      nil,
				LeaderCommit: rf.commitIndex,
			}
			rf.mu.Unlock()

			reply := AppendingEntriesReply{}

			if rf.sendAppendEntries(server, &args, &reply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()

				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.state = StateFollower
					rf.votedFor = -1
					rf.persist()
				}
			}

		}(i)
	}
}

// internal logic of leader election, change self state
// prepare data for broadcasting,...
func (rf *Raft) startElection() {
	// changed internal state/properties, no need for mutex lock
	rf.state = StateCandidate
	rf.currentTerm++
	rf.votedFor = rf.me     // vote for self first
	rf.persist()            // save state
	rf.resetElectionTimer() // reset election time

	term := rf.currentTerm
	votesReceived := 1 // self-voted

	args := RequestVoteArgs{
		Term:         term,
		CandidateId:  rf.me,
		LastLogIndex: len(rf.log) - 1,
		LastLogTerm:  rf.log[len(rf.log)-1].Term,
	}

	// broadcast self information
	for i := range rf.peers {
		if i == rf.me {
			continue
		}

		go func(server int) {
			reply := RequestVoteReply{}

			if rf.sendRequestVote(server, &args, &reply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()

				if rf.currentTerm != term || rf.state != StateCandidate {
					return
				}

				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.state = StateFollower
					rf.votedFor = -1
					rf.persist()
					return
				}

				if reply.VoteGranted {
					votesReceived++
					if rf.state == StateCandidate && votesReceived > len(rf.peers)/2 {
						rf.state = StateLeader

						for i := range rf.peers {
							rf.nextIndex[i] = len(rf.log)
							rf.matchIndex[i] = 0
						}

						// broadcast
						rf.broadcastHeartBeat()
					}
				}
			}
		}(i)
	}
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

// structure of the log entries, as in the paper
// the log entry should only have 2 fields
// one for the command itself (string,...) and
// one for the term that entry received by a node
type LogEntry struct {
	Command interface{}
	Term    int
}

// RPC arguments for appending log entries
type AppendingEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendingEntriesReply struct {
	Term    int
	Success bool
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// RPC appending log handler
func (rf *Raft) AppendEntries(args *AppendingEntriesArgs, reply *AppendingEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()

	// false if incoming term is less than us
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// false if term is different
	// if args.Entries[args.PrevLogIndex].Term

	if args.Term > rf.currentTerm {
		// change to follower (because you're receiving from a "leader")
		rf.currentTerm = args.Term
		rf.state = StateFollower
		rf.votedFor = -1
	}

	rf.lastHeartBeat = time.Now()

	if rf.state == StateCandidate {
		rf.state = StateFollower // force to become a follower even has larger term
	}

	reply.Term = rf.currentTerm
	reply.Success = true

	// log consistent 2B
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	defer rf.persist()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = StateFollower
		rf.votedFor = -1
	}

	reply.Term = rf.currentTerm
	lastLogIndex := len(rf.log) - 1
	lastLogTerm := rf.log[lastLogIndex].Term

	logOK := false

	// check log before finally give vote
	if args.LastLogTerm > lastLogTerm {
		logOK = true
	} else if args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex {
		logOK = true
	}

	// havent vote yet AND candidate log is up-to-date (at least as new as us)
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && logOK {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
		rf.lastHeartBeat = time.Now()
	} else {
		reply.VoteGranted = false
	}
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// RPC call for sending append entries request
// used for send commands from leader to all
// other members in this network
func (rf *Raft) sendAppendEntries(server int, args *AppendingEntriesArgs, reply *AppendingEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// clock for checking election time, hearing from leader,...
func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		rf.mu.Lock()
		if rf.state != StateLeader && time.Since(rf.lastHeartBeat) > rf.electionTimeout {
			// start election process
			rf.startElection()
		}
		rf.mu.Unlock()
		ms := 50 + (rand.Int63() % 300)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	// common state, refer to paper
	rf.currentTerm = 0
	rf.votedFor = -1 // no vote at all (so -1 equals to null in term of int)
	rf.log = make([]LogEntry, 1)
	rf.log[0] = LogEntry{Term: 0}

	// volatile
	rf.commitIndex = 0
	rf.lastApplied = 0

	// state on leader
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	// state
	rf.state = 0 // started as a follower
	rf.lastHeartBeat = time.Now()
	rf.applyCh = applyCh

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
