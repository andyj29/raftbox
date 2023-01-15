package storage

import "sync"

type Persister struct {
	mu        sync.Mutex
	raftState []byte
	snapshot  []byte
}

func (p *Persister) ReadState() []byte {
	return nil
}

func (p *Persister) SaveState(data []byte) {

}

func (p *Persister) SaveStateAndSnapshot(state []byte, snapshot []byte) {

}

func (p *Persister) ReadSnapshot() []byte {
	return nil
}
