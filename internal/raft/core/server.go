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

	// re-initialize non-volatile state from pre-crash
	rs.initNonVolatileState(persister.ReadState())
	rs.applySnapshot(persister.ReadSnapshot())
	return rs
}

func (rs *Server) initNonVolatileState(data []byte) {
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

func (rs *Server) getLastLogIndex() int {
	return rs.log[len(rs.log)-1].Index
}

func (rs *Server) getLastLogTerm() int {
	return rs.log[len(rs.log)-1].Term
}

func (rs *Server) getNonVolatileState() []byte {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)

	if encoder.Encode(rs.currentTerm) != nil ||
		encoder.Encode(rs.votedFor) != nil ||
		encoder.Encode(rs.log) != nil {
		return nil
	}
	return w.Bytes()
}

func (rs *Server) applySnapshot(snapshot []byte) {
	if snapshot == nil || len(snapshot) == 0 {
		return
	}

	var lastIncludedIndex, lastIncludedTerm int
	r := bytes.NewBuffer(snapshot)
	decoder := gob.NewDecoder(r)
	decoder.Decode(&lastIncludedIndex)
	decoder.Decode(&lastIncludedTerm)

	rs.lastApplied = lastIncludedIndex
	rs.commitIndex = lastIncludedIndex

	rs.trimLog(lastIncludedIndex, lastIncludedTerm)
}

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
	rs.storage.SaveState(rs.getNonVolatileState())
}
