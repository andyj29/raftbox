package rpc

import (
	"github.com/andyj29/raftbox/internal/raft/core"
	"github.com/andyj29/raftbox/internal/raft/protobuf"
)

type Client struct {
}

func (c *Client) AppendEntries(request *core.AppendEntryRequest, reply *core.AppendEntryReply) bool {
	req := protobuf.AppendEntryRequest{
		Term:         request.Term,
		LeaderID:     request.LeaderID,
		PrevLogIndex: request.PrevLogIndex,
		PrevLogTerm:  request.PrevLogTerm,
		Entries:      request.Entries,
		LeaderCommit: request.LeaderCommit,
	}
	protobuf.AppendEntries(request, reply)
	return true
}

func (c *Client) RequestVote(request *core.RequestVoteRequest, reply *core.RequestVoteReply) bool {
	return true
}

func (c *Client) InstallSnapshot(request *core.InstallSnapshotRequest, replye *core.InstallSnapshotReply) bool {
	return true
}
