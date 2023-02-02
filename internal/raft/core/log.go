package core

type LogEntry struct {
	Index   int
	Term    int
	Command interface{}
}

type AppendEntryRequest struct {
	Term         int
	LeaderID     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntryReply struct {
	Term         int
	Success      bool
	NextTryIndex int
}

func (rs *Server) AppendEntries(request *AppendEntryRequest, reply *AppendEntryReply) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	defer rs.saveState()

	rs.heartbeat <- true

	reply.Success = false
	reply.Term = rs.currentTerm

	if request.Term < rs.currentTerm {
		reply.NextTryIndex = rs.getLastLogIndex() + 1
		return
	}

	if request.Term > rs.currentTerm {
		rs.abdicateLeadership(request.Term)
		reply.Term = rs.currentTerm
	}

	if request.PrevLogIndex > rs.getLastLogIndex() {
		reply.NextTryIndex = rs.getLastLogIndex() + 1
		return
	}

	baseIndex := rs.log[0].Index
	if request.PrevLogIndex >= baseIndex && request.PrevLogTerm != rs.log[request.PrevLogIndex-baseIndex].Term {
		term := rs.log[request.PrevLogTerm-baseIndex].Term
		for i := request.PrevLogTerm - 1; i >= baseIndex; i-- {
			if rs.log[i-baseIndex].Term != term {
				reply.NextTryIndex = i + 1
				break
			}
		}
	} else if request.PrevLogIndex >= baseIndex-1 {
		rs.log = rs.log[:request.PrevLogIndex+1]
		rs.log = append(rs.log, request.Entries...)
		reply.Success = true
		reply.NextTryIndex = request.PrevLogIndex + len(request.Entries)
		if rs.commitIndex < request.LeaderCommit {
			rs.commitIndex = min(request.LeaderCommit, rs.getLastLogIndex())
			rs.newCond.Broadcast()
		}
	}

}
