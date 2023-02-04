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

func (p *FileStorage) ReadState() core.PersistentState {
	return core.PersistentState{}
}

func (p *FileStorage) SaveState(state core.PersistentState) {

}

func (p *FileStorage) ReadSnapshot() core.Snapshot {
	return core.Snapshot{}
}

func (p *FileStorage) SaveSnapshot(snapshot core.Snapshot) {

}

func (p *FileStorage) SaveStateAndSnapshot(state core.PersistentState, snapshot core.Snapshot) {

}
