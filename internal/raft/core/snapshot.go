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
		rs.stepDown(request.Term)
	}
}

func (rs *Server) TakeSnapshot(stateMachineState map[string]interface{}, index int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	baseIndex, lastIndex := rs.log[0].Index, rs.getLastLogIndex()
	if index <= baseIndex || index > lastIndex {
		return
	}
	rs.trimLog(index, rs.log[index-baseIndex].Term)

	rs.saveState()
	rs.saveSnapshot(stateMachineState)
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
