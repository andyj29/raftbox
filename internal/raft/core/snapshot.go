package core

type InstallSnapshotRequest struct {
	Term     int
	LeaderID int
	Snapshot
}

type InstallSnapshotReply struct {
	Term int
}

type Snapshot struct {
	LastIncludedIndex int
	LastIncludedTerm  int
	StateMachineState map[string]interface{}
}

func (rs *Server) InstallSnapshot(request *InstallSnapshotRequest, reply *InstallSnapshotReply) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	reply.Term = rs.currentTerm
	if request.Term < rs.currentTerm || rs.killed() {
		return
	}

	if request.Term > rs.currentTerm {
		rs.stepDown(request.Term)
		reply.Term = rs.currentTerm
	}

	rs.heartbeat <- true
	if request.LastIncludedIndex > rs.commitIndex {
		rs.commitIndex = request.LastIncludedIndex
		rs.lastApplied = request.LastIncludedIndex
		rs.trimLog(request.LastIncludedIndex, request.LastIncludedTerm)
		rs.storage.SaveStateAndSnapshot(rs.getPersistentState(), request.Snapshot)
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

	snapshot := Snapshot{
		LastIncludedIndex: rs.log[0].Index,
		LastIncludedTerm:  rs.log[0].Term,
		StateMachineState: stateMachineState,
	}
	rs.storage.SaveStateAndSnapshot(rs.getPersistentState(), snapshot)
}

// applySnapshot applies the Raft snapshot and trims the log accordingly
func (rs *Server) applySnapshot(snapshot Snapshot) {
	rs.commitIndex = snapshot.LastIncludedIndex
	rs.lastApplied = snapshot.LastIncludedIndex

	rs.trimLog(snapshot.LastIncludedIndex, snapshot.LastIncludedTerm)
	msg := ApplyMsg{SnapshotValid: true, Snapshot: snapshot}
	rs.applyChan <- msg
}
