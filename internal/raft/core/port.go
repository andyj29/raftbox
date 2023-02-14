package core

type Client interface {
	AppendEntries(request *AppendEntryRequest, reply *AppendEntryReply) bool
	RequestVote(request *RequestVoteRequest, reply *RequestVoteReply) bool
	InstallSnapshot(request *InstallSnapshotRequest, reply *InstallSnapshotReply) bool
}

type Storage interface {
	ReadState() PersistentState
	SaveState(PersistentState)
	ReadSnapshot() Snapshot
	SaveSnapshot(Snapshot)
	SaveStateAndSnapshot(PersistentState, Snapshot)
	Size() int
}
