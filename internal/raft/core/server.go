package core

import (
	"bytes"
	"encoding/gob"
	"sync"

	"github.com/andyj29/raftbox/internal/raft/rpc"
	"github.com/andyj29/raftbox/internal/raft/storage"
)

type STATE int

const (
	LEADER STATE = iota
	CANDIDATE
	FOLLOWER
)

// Server is a struct for a Raft server. It contains the necessary fields for maintaining the
// state of the server, implementing the Raft consensus algorithm, and communicating with
// other servers in the cluster.
type Server struct {
	mu        sync.Mutex
	peers     []*rpc.Client
	storage   *storage.Persister
	state     STATE
	selfIndex int

	currentTerm int
	votedFor    int
	log         []LogEntry

	newCond     *sync.Cond
	commitIndex int
	lastApplied int
	applyChan   chan<- ApplyMsg

	nextIndex  []int
	matchIndex []int
}

// NewRaftServer creates and initializes a new instance of a Raft server,
// applies Raft persistent state and snapshot from persistence
func NewRaftServer(
	peers []*rpc.Client,
	selfIndex int,
	persister *storage.Persister,
	applyChan chan<- ApplyMsg,
) *Server {
	mu := sync.Mutex{}
	rs := &Server{
		mu:          mu,
		peers:       peers,
		storage:     persister,
		state:       FOLLOWER,
		selfIndex:   selfIndex,
		currentTerm: 0,
		votedFor:    -1,
		newCond:     sync.NewCond(&mu),
		commitIndex: 0,
		lastApplied: 0,
		applyChan:   applyChan,
	}

	// initialize Raft persistent state from pre-crash
	rs.initPersistentState(persister.ReadState())
	rs.applySnapshot(persister.ReadSnapshot())
	return rs
}

func (rs *Server) initPersistentState(data []byte) {
	if len(data) == 0 {
		return
	}

	r := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(r)
	var (
		currentTerm, votedFor int
		log                   []LogEntry
	)
	if decoder.Decode(&currentTerm) != nil ||
		decoder.Decode(&votedFor) != nil ||
		decoder.Decode(&log) != nil {
		return
	}
	rs.currentTerm = currentTerm
	rs.votedFor = votedFor
	rs.log = log
}

func (rs *Server) getPersistentState() []byte {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)

	if encoder.Encode(rs.currentTerm) != nil ||
		encoder.Encode(rs.votedFor) != nil ||
		encoder.Encode(rs.log) != nil {
		return nil
	}
	return w.Bytes()
}

// trimLog discards the left-fold entries up to the entry with lastIncludedIndex and keeps
// only its index and term to be referenced by AppendEntryRequest
func (rs *Server) trimLog(lastIncludedIndex, lastIncludedTerm int) {
	newLog := make([]LogEntry, 0)
	newLog = append(newLog, LogEntry{Index: lastIncludedIndex, Term: lastIncludedTerm})

	for i := len(rs.log) - 1; i >= 0; i-- {
		if rs.log[i].Index == lastIncludedIndex && rs.log[i].Term == lastIncludedTerm {
			newLog = append(newLog, rs.log[i+1:]...)
			break
		}

	}
	rs.log = newLog
}

func (rs *Server) saveState() {
	rs.storage.SaveState(rs.getPersistentState())
}

func (rs *Server) getLastLogIndex() int {
	return rs.log[len(rs.log)-1].Index
}

func (rs *Server) getLastLogTerm() int {
	return rs.log[len(rs.log)-1].Term
}

func (rs *Server) isCandidateLogUpToDate(lastLogIndex, lastLogTerm int) bool {
	index, term := rs.getLastLogIndex(), rs.getLastLogTerm()
	return lastLogTerm > term || (lastLogTerm == term && lastLogIndex >= index)
}

// Start is called by the key-value service to start agreement on the next command to be appended
// to Raft log. It returns immediately if the server isn't the leader. Otherwise, it appends the new
// LogEntry to the log and persist the state. Return the index of the command in the log if it's ever committed,
// the current term and whether the server believes it's a leader
func (rs *Server) Start(command interface{}) (index int, term int, isLeader bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	isLeader = rs.state == LEADER
	if !isLeader {
		return
	}

	index = rs.getLastLogIndex() + 1
	term = rs.currentTerm
	rs.log = append(rs.log, LogEntry{Index: index, Term: term, Command: command})
	rs.saveState()

	return index, term, isLeader
}

func (rs *Server) abdicateLeadership(newTerm int) {
	defer rs.saveState()

	rs.state = FOLLOWER
	rs.currentTerm = newTerm
	rs.votedFor = -1
	rs.nextIndex = nil
	rs.matchIndex = nil
}

type RequestVoteRequest struct {
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

func (rs *Server) RequestVote(request *RequestVoteRequest, reply *RequestVoteReply) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	defer rs.saveState()

	reply.Term = rs.currentTerm
	reply.VoteGranted = false

	if request.Term < rs.currentTerm {
		return
	}

	if request.Term > rs.currentTerm {
		rs.abdicateLeadership(request.Term)
	}

	if (rs.votedFor == -1 || rs.votedFor == request.CandidateID) && rs.isCandidateLogUpToDate(request.LastLogIndex, request.LastLogTerm) {
		reply.VoteGranted = true
		rs.votedFor = request.CandidateID
	}
}
