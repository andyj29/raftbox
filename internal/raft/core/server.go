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

type Server struct {
	mu        sync.Mutex
	peers     []*rpc.Client
	storage   *storage.Persister
	state     STATE
	selfIndex int

	currentTerm int
	votedFor    int
	log         []LogEntry

	newCond     sync.Cond
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
	rs := &Server{
		peers:       peers,
		storage:     persister,
		state:       FOLLOWER,
		selfIndex:   selfIndex,
		currentTerm: 0,
		votedFor:    -1,
		commitIndex: 0,
		lastApplied: 0,
		applyChan:   applyChan,
	}

	// re-initialize from pre-crash state
	rs.initState(persister.ReadState())
	return rs
}

func (rs *Server) initState(data []byte) {
	if len(data) == 0 {
		return
	}

	raw := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(raw)
	var (
		currentTerm, votedFor int
		logs                  []LogEntry
	)
	if decoder.Decode(&currentTerm) != nil ||
		decoder.Decode(&votedFor) != nil ||
		decoder.Decode(&logs) != nil {
		return
	}
	rs.currentTerm = currentTerm
	rs.votedFor = votedFor
	rs.log = logs
}
