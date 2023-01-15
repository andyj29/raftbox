package core

import (
	"bytes"
	"encoding/gob"
)

type InstallSnapshotRequest struct {
	Term              int
	LeaderID          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte
}

type InstallSnapshotReply struct {
	Term int
}

func (rs *Server) InstallSnapshot(request *InstallSnapshotRequest, reply *InstallSnapshotReply) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if request.Term < rs.currentTerm {
		reply.Term = rs.currentTerm
		return
	}

	if request.Term > rs.currentTerm {
		rs.abdicateLeadership(request.Term)
	}
}

// TakeSnapshot takes and appends Raft persistent state excluding the log to snapshot,
// from the key-value service and trims the log to the last included index and save
// both updated Raft state and the snapshot as a single atomic action
func (rs *Server) TakeSnapshot(kvSnapshot []byte, index int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	baseIndex, lastIndex := rs.log[0].Index, rs.getLastLogIndex()
	if index <= baseIndex || index > lastIndex {
		return
	}
	rs.trimLog(index, rs.log[index-baseIndex].Term)
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	encoder.Encode(rs.log[0].Index)
	encoder.Encode(rs.log[0].Term)
	raftSnapshot := append(w.Bytes(), kvSnapshot...)
	rs.storage.SaveStateAndSnapshot(rs.getPersistentState(), raftSnapshot)
}

// applySnapshot applies the Raft snapshot and trims the log accordingly
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
