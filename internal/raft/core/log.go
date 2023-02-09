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

	reply.Success = false
	reply.Term = rs.currentTerm

	if request.Term < rs.currentTerm {
		reply.NextTryIndex = rs.getLastLogIndex() + 1
		return
	}

	rs.heartbeat <- true
	if request.Term > rs.currentTerm {
		rs.stepDown(request.Term)
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

func (rs *Server) sendAppendEntriesRPC(server int, request *AppendEntryRequest, reply *AppendEntryReply) (ok bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.state != LEADER || request.Term != rs.currentTerm {
		return ok
	}

	ok = rs.peers[server].AppendEntries(request, reply)
	if reply.Term > rs.currentTerm {
		rs.stepDown(reply.Term)
		return ok
	}

	if reply.Success {
		if len(request.Entries) > 0 {
			rs.nextIndex[server] = request.Entries[len(request.Entries)-1].Index + 1
			rs.matchIndex[server] = rs.nextIndex[server] - 1
		}
	} else {
		rs.nextIndex[server] = min(reply.NextTryIndex, rs.getLastLogIndex())
	}

	baseIndex := rs.log[0].Index

	// check for the log entry with the highest index known to be committed
	// and update leader's commitIndex
	for i := rs.getLastLogIndex(); i > rs.commitIndex && rs.log[i-baseIndex].Term == rs.currentTerm; i-- {
		count := 1
		for s := range rs.peers {
			if s != rs.selfIndex && rs.matchIndex[s] >= i {
				count++
			}
		}
		if count > len(rs.peers)/2 {
			rs.commitIndex = i
			rs.newCond.Broadcast()
			break
		}
	}
	return ok
}
