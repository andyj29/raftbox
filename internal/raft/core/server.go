package core

import (
	"sync"
	"sync/atomic"

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
	storage   *storage.FileStorage
	state     STATE
	selfIndex int
	dead      int32

	PersistentState
	voteCount int

	newCond     *sync.Cond
	commitIndex int
	lastApplied int
	applyChan   chan<- ApplyMsg
	snapChan    chan<- bool

	nextIndex  []int
	matchIndex []int

	heartbeat chan bool
	elected   chan bool
}

type PersistentState struct {
	currentTerm, votedFor int
	log                   []LogEntry
}

// NewRaftServer creates and initializes a new instance of a Raft server,
// applies Raft persistent state and snapshot from persistence
func NewRaftServer(
	peers []*rpc.Client,
	selfIndex int,
	storage *storage.FileStorage,
	applyChan chan<- ApplyMsg,
) *Server {
	mu := sync.Mutex{}
	rs := &Server{}
	rs.mu = mu
	rs.peers = peers
	rs.storage = storage
	rs.state = FOLLOWER
	rs.selfIndex = selfIndex
	rs.votedFor = -1
	rs.newCond = sync.NewCond(&mu)
	rs.applyChan = applyChan

	// initialize Raft persistent state from pre-crash
	rs.initPersistentState(storage.ReadState())
	rs.applySnapshot(storage.ReadSnapshot())
	return rs
}

func (rs *Server) initPersistentState(state PersistentState) {
	rs.currentTerm = state.currentTerm
	rs.votedFor = state.votedFor
	rs.log = state.log
}

func (rs *Server) getPersistentState() PersistentState {
	state := PersistentState{
		currentTerm: rs.currentTerm,
		votedFor:    rs.votedFor,
		log:         rs.log,
	}
	return state
}

// trimLog discards the left-fold entries up to the entry with lastIncludedIndex and keeps
// only its index and term for consistency check
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

	if rs.state != LEADER {
		return
	}

	index = rs.getLastLogIndex() + 1
	term = rs.currentTerm
	rs.log = append(rs.log, LogEntry{Index: index, Term: term, Command: command})
	rs.saveState()

	return index, term, isLeader
}

func (rs *Server) Terminate() {
	atomic.StoreInt32(&rs.dead, 1)
}

func (rs *Server) killed() bool {
	z := atomic.LoadInt32(&rs.dead)
	return z == 1
}

func (rs *Server) stepDown(newTerm int) {
	defer rs.saveState()

	rs.state = FOLLOWER
	rs.currentTerm = newTerm
	rs.votedFor = -1
	rs.nextIndex = nil
	rs.matchIndex = nil
}

func (rs *Server) takeLeadership() {
	rs.state = LEADER
	rs.nextIndex = make([]int, len(rs.peers))
	rs.matchIndex = make([]int, len(rs.peers))
	nextIndex := rs.getLastLogIndex() + 1
	for i := range rs.nextIndex {
		rs.nextIndex[i] = nextIndex
	}
	rs.elected <- true
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
		rs.stepDown(request.Term)
	}

	if (rs.votedFor == -1 || rs.votedFor == request.CandidateID) && rs.isCandidateLogUpToDate(request.LastLogIndex, request.LastLogTerm) {
		reply.VoteGranted = true
		rs.votedFor = request.CandidateID
	}
}

func (rs *Server) sendRequestVoteRPC(server int, request *RequestVoteRequest, reply *RequestVoteReply) (ok bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	defer rs.saveState()

	if rs.state != CANDIDATE || request.Term != rs.currentTerm {
		return ok
	}

	ok = rs.peers[server].RequestVote(request, reply)
	if rs.currentTerm < reply.Term {
		rs.stepDown(reply.Term)
		return ok
	}

	if reply.VoteGranted {
		rs.voteCount++
		if rs.voteCount > len(rs.peers)/2 {
			rs.takeLeadership()
		}
	}

	return ok
}

func (rs *Server) broadcastRequestVoteRPC() {
	rs.mu.Lock()

	request := &RequestVoteRequest{
		Term:         rs.currentTerm,
		CandidateID:  rs.selfIndex,
		LastLogIndex: rs.getLastLogIndex(),
		LastLogTerm:  rs.getLastLogTerm(),
	}
	reply := &RequestVoteReply{}
	rs.mu.Unlock()

	for server := range rs.peers {
		if server != rs.selfIndex {
			go rs.sendRequestVoteRPC(server, request, reply)
		}
	}
}
