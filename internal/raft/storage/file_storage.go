package storage

import (
	"sync"

	"github.com/andyj29/raftbox/internal/raft/core"
)

type FileStorage struct {
	mu        sync.Mutex
	raftState []byte
	snapshot  []byte
}

func (p *FileStorage) ReadState() (int, int, []core.LogEntry) {
	return 0, 0, nil
}

func (p *FileStorage) SaveState(currentTerm, votedFor int, log []core.LogEntry) {
	
}

func (p *FileStorage) SaveSnapshot(lastIncludedIndex, lastIncludedTerm int, stateMachineState map[string]interface{}) {

}

func (p *FileStorage) ReadSnapshot() []byte {
	return nil
}
