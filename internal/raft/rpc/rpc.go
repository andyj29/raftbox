package rpc

import "github.com/andyj29/raftbox/internal/raft/core"

type Client struct {
}

func (c *Client) AppendEntries(request *core.AppendEntryRequest, reply *core.AppendEntryReply) bool {
	return true
}
