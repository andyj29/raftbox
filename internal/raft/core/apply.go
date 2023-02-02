package core

type ApplyMsg struct {
	CommandValid bool
	CommandIndex int
	Command      interface{}

	SnapshotValid bool
	Snapshot      []byte
}

func (rs *Server) watch() {
	defer close(rs.applyChan)
	defer rs.newCond.L.Unlock()

	rs.newCond.L.Lock()
	for !rs.killed() {
		rs.newCond.Wait()

		baseIndex := rs.log[0].Index

		for i := rs.lastApplied + 1; i <= rs.commitIndex; i++ {
			applyMsg := ApplyMsg{
				CommandValid: true,
				CommandIndex: rs.lastApplied + 1,
				Command:      rs.log[i-baseIndex].Command,
			}
			rs.applyChan <- applyMsg
			rs.lastApplied++
		}
	}
}
