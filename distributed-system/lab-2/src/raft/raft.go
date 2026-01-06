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
	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labgob"
	"6.5840/labrpc"
)

// numerical comparison for state of a node
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

// helper functions for snapshot - start

// Get the logic index of the last log entry
func (rf *Raft) getLastLogIndex() int {
	return rf.lastIncludedIndex + len(rf.log) - 1
}

// Get the term of the last log entry
func (rf *Raft) getLastLogTerm() int {
	if len(rf.log) == 0 {
		return rf.lastIncludedTerm
	}
	return rf.log[len(rf.log)-1].Term
}

// Helper to get term by logical index
func (rf *Raft) getLogTerm(logicalIndex int) int {
	if logicalIndex < rf.lastIncludedIndex {
		return -1 // Should not happen in normal logic
	}
	if logicalIndex == rf.lastIncludedIndex {
		return rf.lastIncludedTerm
	}
	return rf.log[logicalIndex-rf.lastIncludedIndex].Term
}

// helper functions for snapshot - end

// reset timer each time (increase term if follower, reset to become candidate)
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

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 {
		return
	}

	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)

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
}

// broadcast Heartbeat of the leader
// frequently send this to all follower
func (rf *Raft) broadcastHeartBeat() {
	for i := range rf.peers {
		if i == rf.me {
			continue
		}

		// start with go routine (concurrently)
		go func(server int) {
			rf.mu.Lock()
			if rf.state != StateLeader {
				rf.mu.Unlock()
				return
			}

			// calculate lastest index of this follower
			prevLogIndex := rf.nextIndex[server] - 1

			// IMPORTANT: If the follower needs a log entry that we have discarded...
			if prevLogIndex < rf.lastIncludedIndex {
				rf.mu.Unlock()
				rf.sendInstallSnapshot(server)
				return
			}

			// Normal AppendEntries logic
			// Calculate physical index
			prevLogTerm := rf.getLogTerm(prevLogIndex)

			// send entries
			var entries []LogEntry
			// if leader has more log than follower
			if rf.getLastLogIndex() > prevLogIndex {
				// Copy entries starting from nextIndex
				// Physical index = nextIndex - lastIncludedIndex
				nextPhys := rf.nextIndex[server] - rf.lastIncludedIndex
				if nextPhys < 1 {
					nextPhys = 1 // never send dummy entry at rf.log[0]
				}
				entries = make([]LogEntry, len(rf.log[nextPhys:]))
				copy(entries, rf.log[nextPhys:])
			}

			args := AppendingEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: rf.commitIndex,
			}
			rf.mu.Unlock()

			reply := AppendingEntriesReply{}
			if rf.sendAppendEntries(server, &args, &reply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()

				// check reply term, if larger, voluntary give up leadership
				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.state = StateFollower
					rf.votedFor = -1
					rf.persist()
					return
				}

				if rf.state != StateLeader {
					return
				}

				if reply.Success {
					// if reply was success, update matchIndex (sent)
					// and nextIndex (for next time)
					newMatchIndex := args.PrevLogIndex + len(args.Entries) // previous + sent
					if newMatchIndex > rf.matchIndex[server] {
						rf.matchIndex[server] = newMatchIndex
						rf.nextIndex[server] = rf.matchIndex[server] + 1

						// TODO heck commit status
						rf.checkCommit()
					}
				} else {
					// not successfully got reply, maybe due to out-of-date log
					// or we are not leader anymore,... simply jump 1 index
					// set to 1 is negative
					// rf.nextIndex[server]--
					// if rf.nextIndex[server] < 1 {
					// 	rf.nextIndex[server] = 1
					// }
					// handle confict
					if reply.ConflictTerm == -1 {
						// no term conflict
						rf.nextIndex[server] = reply.ConflictIndex
					} else {
						// find what term is conflict
						lastIndexOfTerm := -1
						for i := len(rf.log) - 1; i >= 0; i-- {
							if rf.log[i].Term == reply.ConflictTerm {
								lastIndexOfTerm = i
								break
							}
						}

						if lastIndexOfTerm != -1 {
							// found
							rf.nextIndex[server] = (lastIndexOfTerm + rf.lastIncludedIndex) + 1
						} else {
							rf.nextIndex[server] = reply.ConflictIndex
						}
					}
				}
			}
		}(i)
	}
}

// check commit, quorum for commit, if majority of the peers have this term
// commit it, otherwise, discard
func (rf *Raft) checkCommit() {
	if rf.state != StateLeader {
		return
		// only perform if leader
	}

	for N := rf.getLastLogIndex(); N > rf.commitIndex; N-- {
		// looking from end down to last commit
		// check each entry and apply

		if N <= rf.lastIncludedIndex {
			break
		}

		count := 1
		for i := range rf.peers {
			if i != rf.me && rf.matchIndex[i] >= N {
				count++ // count if server i was commited (up to date)
			}
		}

		if count > len(rf.peers)/2 && rf.getLogTerm(N) == rf.currentTerm {
			// if majority agrees and term is match (up-to-date)
			rf.commitIndex = N
			rf.applyCond.Broadcast() // notify
			break                    // only check for the highest vaule of N
		}
	}
}

// internal logic of leader election, change self state
// prepare data for broadcasting,...
func (rf *Raft) startElection() {
	// changed internal state/properties, no need for mutex lock
	rf.state = StateCandidate
	rf.currentTerm++        // increase term (cause it is still running)
	rf.votedFor = rf.me     // vote for self first
	rf.persist()            // save state
	rf.resetElectionTimer() // reset election time

	term := rf.currentTerm
	votesReceived := 1 // self-voted

	lastIndex := rf.getLastLogIndex()
	lastTerm := rf.getLastLogTerm()

	args := RequestVoteArgs{
		Term:         term,
		CandidateId:  rf.me,
		LastLogIndex: lastIndex,
		LastLogTerm:  lastTerm,
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
					// if timeout (split vote,...) and term was increased before receiving reply
					// and somehow, it does not retain candidate state
					return
				}

				// found a newer node
				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.state = StateFollower
					rf.votedFor = -1
					rf.persist()
					return
				}

				// successfully received vote
				if reply.VoteGranted {
					votesReceived++
					if rf.state == StateCandidate && votesReceived > len(rf.peers)/2 {
						rf.state = StateLeader

						last := rf.getLastLogIndex()
						for i := range rf.peers {
							rf.nextIndex[i] = last + 1
							rf.matchIndex[i] = rf.lastIncludedIndex
						}
						rf.matchIndex[rf.me] = last

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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// If the requested index is older than or equal to the current snapshot, ignore it.
	if index <= rf.lastIncludedIndex {
		return
	}

	// Protection against invalid index (though unlikely if service is correct)
	if index > rf.getLastLogIndex() {
		index = rf.getLastLogIndex()
	}

	// 1. Calculate the offset in the current log slice
	// Current log logic: rf.log[0] is the dummy entry at rf.lastIncludedIndex
	// We want to discard up to 'index'.
	// The physical index in the slice is: index - rf.lastIncludedIndex
	cutoffOffset := index - rf.lastIncludedIndex

	// 2. Save the term of the entry at the snapshot boundary
	rf.lastIncludedTerm = rf.log[cutoffOffset].Term
	rf.lastIncludedIndex = index

	// 3. Trim the log
	// We create a new log slice. The first entry must be a dummy entry
	// containing the Term and Index of the snapshot.
	newLog := make([]LogEntry, 0)
	newLog = append(newLog, LogEntry{Term: rf.lastIncludedTerm, Command: nil})

	// Append the log entries following the snapshot index
	// We take everything after cutoffOffset
	if cutoffOffset+1 < len(rf.log) {
		newLog = append(newLog, rf.log[cutoffOffset+1:]...)
	}

	rf.log = newLog

	// 4. Persist both Raft state and the Snapshot
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)

	// SaveStateAndSnapshot: atomically save both
	rf.persister.Save(w.Bytes(), snapshot)

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
	Term          int
	ConflictTerm  int
	ConflictIndex int
	Success       bool
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

type InstallSnapshotArgs struct {
	Term              int
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte
}

type InstallSnapshotReply struct {
	Term int
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

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

// example RequestVote RPC handler.
// handler a vote request from upcoming message
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	defer rf.persist()

	// dont vote if coming < us
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// convert to follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = StateFollower
		rf.votedFor = -1
	}

	// send self-infomation with reply
	reply.Term = rf.currentTerm

	lastLogIndex := rf.getLastLogIndex()
	lastLogTerm := rf.getLastLogTerm()

	logOK := false

	// check log before finally give vote
	// log consistency
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

func (rf *Raft) sendInstallSnapshot(server int) {
	rf.mu.Lock()
	if rf.state != StateLeader {
		rf.mu.Unlock()
		return
	}
	args := InstallSnapshotArgs{
		Term:              rf.currentTerm,
		LeaderId:          rf.me,
		LastIncludedIndex: rf.lastIncludedIndex,
		LastIncludedTerm:  rf.lastIncludedTerm,
		Data:              rf.persister.ReadSnapshot(),
	}
	rf.mu.Unlock()

	reply := InstallSnapshotReply{}
	ok := rf.peers[server].Call("Raft.InstallSnapshot", &args, &reply)
	if !ok {
		return
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.state = StateFollower
		rf.votedFor = -1
		rf.persist()
		return
	}

	if rf.state != StateLeader || rf.currentTerm != args.Term {
		return
	}

	if rf.matchIndex[server] < args.LastIncludedIndex {
		rf.matchIndex[server] = args.LastIncludedIndex
	}
	if rf.nextIndex[server] < args.LastIncludedIndex+1 {
		rf.nextIndex[server] = args.LastIncludedIndex + 1
	}

	rf.checkCommit()
	go rf.broadcastHeartBeat()
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm

	if args.Term < rf.currentTerm {
		return
	}

	// If we discover a new leader (or same leader with higher term)
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = StateFollower
		rf.votedFor = -1
		rf.persist()
	}
	reply.Term = rf.currentTerm

	if rf.state == StateCandidate {
		rf.state = StateFollower
	}

	// Reset election timer (heartbeat logic)
	// rf.lastHeartBeat = time.Now()
	rf.resetElectionTimer() // more stable

	// 1. Check if the snapshot is older than what we already have
	if args.LastIncludedIndex <= rf.lastIncludedIndex {
		return
	}

	// 2. Check if our current log contains the snapshot's last index
	// and if the terms match. If so, we can preserve part of the log.
	hasEntry := false
	if args.LastIncludedIndex <= rf.getLastLogIndex() {
		if rf.getLogTerm(args.LastIncludedIndex) == args.LastIncludedTerm {
			hasEntry = true
		}
	}

	if hasEntry {
		// We have the entry, so we keep the log after that index
		// Calculate the relative index in the current slice
		physicalOffset := args.LastIncludedIndex - rf.lastIncludedIndex

		// Create new log with dummy entry
		newLog := make([]LogEntry, 0)
		newLog = append(newLog, LogEntry{Term: args.LastIncludedTerm})

		// Copy remaining entries
		if physicalOffset+1 < len(rf.log) {
			newLog = append(newLog, rf.log[physicalOffset+1:]...)
		}
		rf.log = newLog
	} else {
		// Log is totally behind or conflicting, discard all
		rf.log = make([]LogEntry, 1)
		rf.log[0] = LogEntry{Term: args.LastIncludedTerm}
	}

	// 3. Update state
	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.lastIncludedTerm = args.LastIncludedTerm

	// Update commitIndex if necessary
	if rf.commitIndex < args.LastIncludedIndex {
		rf.commitIndex = args.LastIncludedIndex
	}

	// 4. Persist
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)
	rf.persister.Save(w.Bytes(), args.Data)

	// 5. Apply the snapshot to the service
	rf.applyCond.Broadcast()
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != StateLeader {
		return -1, -1, false
	}

	// Your code here (2B).
	// Use logic index, not slice length
	index := rf.getLastLogIndex() + 1
	term := rf.currentTerm

	rf.log = append(rf.log, LogEntry{Term: rf.currentTerm, Command: command})
	rf.persist()

	rf.broadcastHeartBeat()

	return index, term, true
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
// used the randomized to ensure every nodes have different wait time
// reset the period each term to avoid split votes
func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		rf.mu.Lock()

		// snapshot of currentstate
		state := rf.state

		if state == StateLeader {
			// leader need to broadcast frequently
			if time.Since(rf.lastBroadcastTime) > 100*time.Millisecond {
				// using 100 since election in range [300,450] with 150 interval
				rf.lastBroadcastTime = time.Now()
				rf.mu.Unlock()
				rf.broadcastHeartBeat()
				rf.mu.Lock()
			}
		}

		if rf.state != StateLeader && time.Since(rf.lastHeartBeat) > rf.electionTimeout {
			// start election process
			// if currently not leader and not hear from leader affter a time (candidate logic)

			rf.startElection()
		}
		rf.mu.Unlock()

		// sleep again
		// ms := 50 + (rand.Int63() % 300)
		time.Sleep(10 * time.Millisecond)
	}
}

// for the log consistency, the apply log should be handled separately
// as suggested in the paper, the log (if missing) on the leader should
// be later transferred,...
func (rf *Raft) applier() {
	for rf.killed() == false {
		rf.mu.Lock()
		for rf.lastApplied >= rf.commitIndex && rf.lastApplied >= rf.lastIncludedIndex {
			// already apply latest log, wait for new
			rf.applyCond.Wait()
		}

		// check whether need to send snapshot
		if rf.lastApplied < rf.lastIncludedIndex {
			snapshotMsg := ApplyMsg{
				SnapshotValid: true,
				Snapshot:      rf.persister.ReadSnapshot(),
				SnapshotTerm:  rf.lastIncludedTerm,
				SnapshotIndex: rf.lastIncludedIndex,
			}

			newLastApplied := rf.lastIncludedIndex

			rf.mu.Unlock()

			rf.applyCh <- snapshotMsg

			rf.mu.Lock()
			if newLastApplied > rf.lastApplied {
				rf.lastApplied = newLastApplied
			}
			rf.mu.Unlock()
			continue
		}

		// if there is something that needs to be updated
		firstIndex := rf.lastApplied + 1
		lastIndex := rf.commitIndex
		// check boundary（to avoid shapshot cause condition: firstIndex < lastIncludedIndex）
		if firstIndex <= rf.lastIncludedIndex {
			firstIndex = rf.lastIncludedIndex + 1
		}

		entries := make([]LogEntry, 0)

		if firstIndex <= lastIndex {
			start := firstIndex - rf.lastIncludedIndex
			end := lastIndex - rf.lastIncludedIndex + 1

			if start < len(rf.log) && end <= len(rf.log) {
				tempEntries := make([]LogEntry, end-start)
				copy(tempEntries, rf.log[start:end])
				entries = tempEntries
			}
		}

		rf.mu.Unlock()

		// reply client
		for i, entry := range entries {
			msg := ApplyMsg{
				CommandValid: true,
				Command:      entry.Command,
				CommandIndex: firstIndex + i,
			}
			rf.applyCh <- msg
		}

		// update last apply after successfully send message
		rf.mu.Lock()
		if lastIndex > rf.lastApplied {
			rf.lastApplied = lastIndex
		}
		rf.mu.Unlock()
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
	rf.applyCond = sync.NewCond(&rf.mu)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	if rf.lastIncludedIndex > 0 {
		rf.lastApplied = rf.lastIncludedIndex
		rf.commitIndex = rf.lastIncludedIndex
	}

	// start ticker goroutine to start elections
	go rf.ticker()

	// start the applier goroutine
	go rf.applier()

	return rf
}
